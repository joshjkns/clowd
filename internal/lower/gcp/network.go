package gcp

import (
	"net/netip"

	"github.com/joshjkns/clowd/internal/emit/tfjson"
	"github.com/joshjkns/clowd/internal/ir"
)

// defaultCIDR is the network's block when the program sets no `cidr`.
const defaultCIDR = "10.0.0.0/16"

// network holds references to the VPC; empty when there is none.
type network struct {
	id, subnet tfjson.Expr
	// peering is the private services connection Cloud SQL needs.
	peering string
	// proxy is the proxy-only subnet internal load balancers need.
	proxy string
}

// network creates the VPC: a subnet that services send traffic through
// (Direct VPC egress) and internal load balancers listen in, a range
// peered with Google's for Cloud SQL's private addresses, a proxy-only
// subnet for internal load balancers, and Cloud NAT for a private
// network's services.
//
// The program's block is split into sixteenths: the first for the
// subnet, the second for Cloud SQL, the third for load balancer proxies.
// A /16 gives each a /20, and a /20 (the smallest `cidr` allows) a /24,
// which is still the least each of them accepts.
func (l *lowerer) network() {
	n := l.p.Network
	cidr := n.CIDR
	if cidr == "" {
		cidr = defaultCIDR
	}
	block := netip.MustParsePrefix(cidr)
	vpc := l.cfg.Resource("google_compute_network", "main", tfjson.Body{
		"name":                    l.physical(63),
		"auto_create_subnetworks": false,
	})
	subnet := l.cfg.Resource("google_compute_subnetwork", "main", tfjson.Body{
		"name":                     l.physical(63),
		"network":                  ref(vpc, "id"),
		"region":                   l.loc,
		"ip_cidr_range":            sixteenth(block, 0).String(),
		"private_ip_google_access": true,
	})
	l.net = network{id: ref(vpc, "id"), subnet: ref(subnet, "id")}

	if l.has(ir.DatabaseKind) {
		r := sixteenth(block, 1)
		rng := l.cfg.Resource("google_compute_global_address", "private_services", tfjson.Body{
			"name":          l.physical(63, "private-services"),
			"network":       ref(vpc, "id"),
			"purpose":       "VPC_PEERING",
			"address_type":  "INTERNAL",
			"address":       r.Addr().String(),
			"prefix_length": r.Bits(),
		})
		l.net.peering = l.cfg.Resource("google_service_networking_connection", "private_services", tfjson.Body{
			"network":                 ref(vpc, "id"),
			"service":                 "servicenetworking.googleapis.com",
			"reserved_peering_ranges": []tfjson.Expr{ref(rng, "name")},
			// Google releases the peering some time after the last
			// Cloud SQL instance is deleted, so deleting it in the same
			// destroy fails; it goes with the network instead.
			"deletion_policy": "ABANDON",
		})
	}
	if hasInternalLB(l.p) {
		l.net.proxy = l.cfg.Resource("google_compute_subnetwork", "proxy", tfjson.Body{
			"name":          l.physical(63, "proxy"),
			"network":       ref(vpc, "id"),
			"region":        l.loc,
			"ip_cidr_range": sixteenth(block, 2).String(),
			"purpose":       "REGIONAL_MANAGED_PROXY",
			"role":          "ACTIVE",
		})
	}
	if n.Private && l.has(ir.ServiceKind) {
		router := l.cfg.Resource("google_compute_router", "main", tfjson.Body{
			"name":    l.physical(63),
			"network": ref(vpc, "id"),
			"region":  l.loc,
		})
		l.cfg.Resource("google_compute_router_nat", "main", tfjson.Body{
			"name":                               l.physical(63),
			"router":                             ref(router, "name"),
			"region":                             l.loc,
			"nat_ip_allocate_option":             "AUTO_ONLY",
			"source_subnetwork_ip_ranges_to_nat": "ALL_SUBNETWORKS_ALL_IP_RANGES",
		})
	}
}

// hasInternalLB reports whether p has an internal load balancer.
func hasInternalLB(p *ir.Program) bool {
	for _, r := range p.Resources {
		if lb, ok := r.(*ir.LoadBalancer); ok && lb.Internal {
			return true
		}
	}
	return false
}

// sixteenth returns the i'th of the sixteen blocks p splits into.
func sixteenth(p netip.Prefix, i int) netip.Prefix {
	bits := p.Bits() + 4
	a := p.Masked().Addr().As4()
	v := uint32(a[0])<<24 | uint32(a[1])<<16 | uint32(a[2])<<8 | uint32(a[3])
	v += uint32(i) << (32 - bits)
	return netip.PrefixFrom(netip.AddrFrom4([4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}), bits)
}
