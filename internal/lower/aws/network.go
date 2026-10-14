package aws

import (
	"fmt"

	"github.com/joshjkns/clowd/internal/emit/tfjson"
	"github.com/joshjkns/clowd/internal/ir"
)

// defaultCIDR is the VPC block when Options.Network.CIDR is empty.
const defaultCIDR = "10.0.0.0/16"

// zones is how many availability zones the created network spans: two is
// the least an ALB and an RDS subnet group accept.
const zones = 2

// network places the program. With an existing VPC it only looks the VPC
// up for its CIDR block. Otherwise it creates one with a public and a
// private subnet in each of two zones: load balancers (and, by default,
// services) go in the public subnets, databases in the private ones.
//
// Public services get public IPs so they can pull images and call AWS APIs
// without a NAT gateway, which costs more than a small service; their
// security groups only admit their load balancers. Private services need
// the NAT gateway, so it is created only for them.
func (l *lowerer) network() {
	n := l.opts.Network
	if n.VPC != "" {
		vpc := l.cfg.DataSource("aws_vpc", "main", tfjson.Body{"id": n.VPC})
		l.net = network{vpc: n.VPC, cidr: ref(vpc, "cidr_block"), public: n.PublicSubnets, private: n.PrivateSubnets}
		return
	}
	cidr := n.CIDR
	if cidr == "" {
		cidr = defaultCIDR
	}
	az := l.cfg.DataSource("aws_availability_zones", "available", tfjson.Body{"state": "available"})
	vpc := l.cfg.Resource("aws_vpc", "main", tfjson.Body{
		"cidr_block":           cidr,
		"enable_dns_support":   true,
		"enable_dns_hostnames": true,
		"tags":                 tags(l.opts.Project),
	})
	igw := l.cfg.Resource("aws_internet_gateway", "main", tfjson.Body{
		"vpc_id": ref(vpc, "id"),
		"tags":   tags(l.opts.Project),
	})
	// Subnets are /24s: public ones from 10.0.0.0, private ones from
	// 10.0.10.0, leaving room to add zones.
	subnet := func(name string, offset int) string {
		return l.cfg.Resource("aws_subnet", name, tfjson.Body{
			"count":             zones,
			"vpc_id":            ref(vpc, "id"),
			"cidr_block":        tfjson.Expr(fmt.Sprintf("${cidrsubnet(aws_vpc.main.cidr_block, 8, count.index + %d)}", offset)),
			"availability_zone": ref(az, "names[count.index]"),
			"tags":              tags(l.physical(255, name)),
		})
	}
	public, private := subnet("public", 0), subnet("private", 10)

	routes := func(name, subnets string) string {
		table := l.cfg.Resource("aws_route_table", name, tfjson.Body{
			"vpc_id": ref(vpc, "id"),
			"tags":   tags(l.physical(255, name)),
		})
		l.cfg.Resource("aws_route_table_association", name, tfjson.Body{
			"count":          zones,
			"subnet_id":      tfjson.Ref(subnets + "[count.index].id"),
			"route_table_id": ref(table, "id"),
		})
		return table
	}
	publicRoutes := routes("public", public)
	l.cfg.Resource("aws_route", "public_internet", tfjson.Body{
		"route_table_id":         ref(publicRoutes, "id"),
		"destination_cidr_block": "0.0.0.0/0",
		"gateway_id":             ref(igw, "id"),
	})
	privateRoutes := routes("private", private)
	if n.PrivateServices && l.has(ir.ServiceKind) {
		eip := l.cfg.Resource("aws_eip", "nat", tfjson.Body{
			"domain": "vpc",
			"tags":   tags(l.physical(255, "nat")),
		})
		nat := l.cfg.Resource("aws_nat_gateway", "main", tfjson.Body{
			"allocation_id": ref(eip, "id"),
			"subnet_id":     tfjson.Ref(public + "[0].id"),
			"tags":          tags(l.opts.Project),
			"depends_on":    []string{igw},
		})
		l.cfg.Resource("aws_route", "private_internet", tfjson.Body{
			"route_table_id":         ref(privateRoutes, "id"),
			"destination_cidr_block": "0.0.0.0/0",
			"nat_gateway_id":         ref(nat, "id"),
		})
	}
	l.net = network{
		vpc:     ref(vpc, "id"),
		cidr:    ref(vpc, "cidr_block"),
		public:  tfjson.Ref(public + "[*].id"),
		private: tfjson.Ref(private + "[*].id"),
	}
}

// serviceSubnets are where services run.
func (l *lowerer) serviceSubnets() any {
	if l.opts.Network.PrivateServices {
		return l.net.private
	}
	return l.net.public
}
