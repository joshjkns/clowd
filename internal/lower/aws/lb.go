package aws

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/joshjkns/clowd/internal/emit/tfjson"
	"github.com/joshjkns/clowd/internal/ir"
)

// tlsPolicy is the listener security policy: TLS 1.3, falling back to 1.2.
const tlsPolicy = "ELBSecurityPolicy-TLS13-1-2-2021-06"

func targetGroupAddr(lb, service string) string {
	return "aws_lb_target_group." + id(lb) + "_" + id(service)
}

func listenerAddr(lb string, port int) string {
	return fmt.Sprintf("aws_lb_listener.%s_%d", id(lb), port)
}

// loadBalancer lowers lb to an Application Load Balancer.
//
// Each `send` target gets a target group and each `listen` a listener;
// the `send /` route is the listener's default action and every other path
// is a rule, longest path first so the most specific prefix wins. A
// listener with no `/` route answers 404 to unmatched paths.
//
// A TLS listener gets an ACM certificate validated by DNS. Nobody but the
// domain's owner can create the validation record, so the certificate is
// reported in Result.Certificates for the runner to create first.
func (l *lowerer) loadBalancer(lb *ir.LoadBalancer) {
	n := id(lb.Name)
	sg := l.securityGroup(n, "clowd load balancer "+lb.Name)
	source := map[string]any{"cidr_ipv4": "0.0.0.0/0"}
	if lb.Internal {
		source = map[string]any{"cidr_ipv4": l.net.cidr}
	}
	for _, ln := range lb.Listeners {
		body := tfjson.Body{
			"security_group_id": ref(sg, "id"),
			"description":       fmt.Sprintf("listen on %d", ln.Port),
			"ip_protocol":       "tcp",
			"from_port":         ln.Port,
			"to_port":           ln.Port,
		}
		for k, v := range source {
			body[k] = v
		}
		l.cfg.Resource("aws_vpc_security_group_ingress_rule", fmt.Sprintf("%s_%d", n, ln.Port), body)
	}

	subnets := l.net.public
	if lb.Internal {
		subnets = l.net.private
	}
	alb := l.cfg.Resource("aws_lb", n, tfjson.Body{
		"name":                       l.physical(32, n),
		"load_balancer_type":         "application",
		"internal":                   lb.Internal,
		"subnets":                    subnets,
		"security_groups":            []tfjson.Expr{ref(sg, "id")},
		"drop_invalid_header_fields": true,
	})

	var services []string
	for _, rt := range lb.Routes {
		if !slices.Contains(services, rt.Service) {
			services = append(services, rt.Service)
		}
	}
	for _, name := range services {
		s, ok := l.p.Resource(name).(*ir.Service)
		if !ok {
			continue
		}
		l.cfg.Resource("aws_vpc_security_group_egress_rule", n+"_to_"+id(name), tfjson.Body{
			"security_group_id":            ref(sg, "id"),
			"description":                  "to service " + name,
			"referenced_security_group_id": ref("aws_security_group."+id(name), "id"),
			"ip_protocol":                  "tcp",
			"from_port":                    s.Port,
			"to_port":                      s.Port,
		})
		// Without a health check path, any answer below 500 at `/` means
		// the service is up, since a 404 there is normal for an API. A
		// path the user named is a health endpoint, so it must succeed.
		path, matcher := "/", "200-499"
		if s.HealthCheck != "" {
			path, matcher = s.HealthCheck, "200-399"
		}
		_, tg := splitAddr(targetGroupAddr(lb.Name, name))
		l.cfg.Resource("aws_lb_target_group", tg, tfjson.Body{
			"name":                 l.physical(32, n, id(name)),
			"port":                 s.Port,
			"protocol":             "HTTP",
			"target_type":          "ip",
			"vpc_id":               l.net.vpc,
			"deregistration_delay": 30,
			"health_check": tfjson.Body{
				"path":                path,
				"matcher":             matcher,
				"interval":            15,
				"healthy_threshold":   2,
				"unhealthy_threshold": 3,
			},
		})
	}

	forward := func(service string) tfjson.Body {
		return tfjson.Body{"type": "forward", "target_group_arn": ref(targetGroupAddr(lb.Name, service), "arn")}
	}
	var root *ir.Route
	var rules []ir.Route
	for i, rt := range lb.Routes {
		if rt.Path == "/" {
			root = &lb.Routes[i]
		} else {
			rules = append(rules, rt)
		}
	}
	slices.SortStableFunc(rules, func(a, b ir.Route) int { return cmp.Compare(len(b.Path), len(a.Path)) })

	certs := map[string]string{}
	for _, ln := range lb.Listeners {
		body := tfjson.Body{
			"load_balancer_arn": ref(alb, "arn"),
			"port":              ln.Port,
			"protocol":          "HTTP",
		}
		if ln.TLS() {
			cert, ok := certs[ln.Domain]
			if !ok {
				cert = l.certificate(n, ln.Domain)
				certs[ln.Domain] = cert
			}
			body["protocol"] = "HTTPS"
			body["ssl_policy"] = tlsPolicy
			body["certificate_arn"] = ref(cert, "certificate_arn")
		}
		if root != nil {
			body["default_action"] = forward(root.Service)
		} else {
			body["default_action"] = tfjson.Body{
				"type": "fixed-response",
				"fixed_response": tfjson.Body{
					"content_type": "text/plain",
					"message_body": "not found",
					"status_code":  "404",
				},
			}
		}
		_, name := splitAddr(listenerAddr(lb.Name, ln.Port))
		listener := l.cfg.Resource("aws_lb_listener", name, body)
		for i, rt := range rules {
			l.cfg.Resource("aws_lb_listener_rule", fmt.Sprintf("%s_%d", name, i+1), tfjson.Body{
				"listener_arn": ref(listener, "arn"),
				"priority":     i + 1,
				"action":       forward(rt.Service),
				// Paths are prefixes: `/admin` matches `/admin`,
				// `/admin/users` and `/administrator`.
				"condition": tfjson.Body{"path_pattern": tfjson.Body{"values": []string{rt.Path + "*"}}},
			})
		}
	}

	l.cfg.Outputs = append(l.cfg.Outputs, tfjson.Output{
		Name:        n + "_dns_name",
		Description: "The DNS name of load balancer " + lb.Name + "; point your domains at it with a CNAME record",
		Value:       ref(alb, "dns_name"),
	})
	if len(lb.Listeners) > 0 {
		l.cfg.Outputs = append(l.cfg.Outputs, tfjson.Output{
			Name:        n + "_url",
			Description: "Where load balancer " + lb.Name + " serves",
			Value:       url(lb.Listeners[0], ref(alb, "dns_name")),
		})
	}
}

// url is where a listener can be reached: its domain for TLS, otherwise
// the load balancer's own name.
func url(ln ir.Listener, dnsName tfjson.Expr) tfjson.Expr {
	if ln.TLS() {
		u := "https://" + tfjson.Escape(ln.Domain)
		if ln.Port != 443 {
			u += fmt.Sprintf(":%d", ln.Port)
		}
		return tfjson.Expr(u)
	}
	u := "http://" + string(dnsName)
	if ln.Port != 80 {
		u += fmt.Sprintf(":%d", ln.Port)
	}
	return tfjson.Expr(u)
}

// certificate creates a DNS-validated certificate for domain and returns
// the address of its validation, which completes once the record exists.
func (l *lowerer) certificate(lb, domain string) string {
	name := lb + "_" + slug(domain)
	cert := l.cfg.Resource("aws_acm_certificate", name, tfjson.Body{
		"domain_name":       domain,
		"validation_method": "DNS",
		// A replacement must exist before the listener lets go of the old one.
		"lifecycle": tfjson.Body{"create_before_destroy": true},
	})
	l.certs = append(l.certs, cert)
	l.cfg.Outputs = append(l.cfg.Outputs, tfjson.Output{
		Name:        name + "_validation",
		Description: "The DNS record that proves you own " + domain,
		Value:       ref(cert, "domain_validation_options"),
	})
	return l.cfg.Resource("aws_acm_certificate_validation", name, tfjson.Body{
		"certificate_arn": ref(cert, "arn"),
	})
}

// slug turns a domain into an identifier: `*.example.com` → `wildcard_example_com`.
func slug(domain string) string {
	domain = strings.Replace(domain, "*", "wildcard", 1)
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '_'
	}, strings.ToLower(domain))
}

// splitAddr splits "type.name".
func splitAddr(addr string) (typ, name string) {
	typ, name, _ = strings.Cut(addr, ".")
	return typ, name
}
