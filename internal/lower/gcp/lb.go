package gcp

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/emit/tfjson"
	"github.com/joshjkns/clowd/internal/ir"
)

// lbTypes are the resource types of one flavour of Application Load
// Balancer: global for public ones, regional for internal ones, which
// live in the network.
type lbTypes struct {
	scheme                                 string
	backend, urlMap, httpProxy, httpsProxy string
	forwardingRule, address                string
	regional                               bool
}

var (
	publicLB = lbTypes{
		scheme:         "EXTERNAL_MANAGED",
		backend:        "google_compute_backend_service",
		urlMap:         "google_compute_url_map",
		httpProxy:      "google_compute_target_http_proxy",
		httpsProxy:     "google_compute_target_https_proxy",
		forwardingRule: "google_compute_global_forwarding_rule",
		address:        "google_compute_global_address",
	}
	internalLB = lbTypes{
		scheme:         "INTERNAL_MANAGED",
		backend:        "google_compute_region_backend_service",
		urlMap:         "google_compute_region_url_map",
		httpProxy:      "google_compute_region_target_http_proxy",
		httpsProxy:     "google_compute_region_target_https_proxy",
		forwardingRule: "google_compute_forwarding_rule",
		address:        "google_compute_address",
		regional:       true,
	}
)

// flavour returns the types for a public or internal load balancer.
func flavour(internal bool) lbTypes {
	if internal {
		return internalLB
	}
	return publicLB
}

// loadBalancer lowers lb to an Application Load Balancer.
//
// Each `send` target gets a serverless endpoint group and a backend
// service, and the URL map routes to them: the `send /` route is the
// default and every other path is a prefix rule, longest first, so the
// most specific prefix wins. Each `listen` gets a proxy and a forwarding
// rule, all on one address.
//
// A TLS listener gets a Certificate Manager certificate, authorised by a
// DNS record that only the domain's owner can create. Unlike ACM, the
// apply doesn't wait for it: the certificate is issued once the record
// exists, so the record is an output that `clowd apply` prints.
func (l *lowerer) loadBalancer(lb *ir.LoadBalancer) {
	n := id(lb.Name)
	t := flavour(lb.Internal)
	regional := func(body tfjson.Body) tfjson.Body {
		if t.regional {
			body["region"] = l.loc
		}
		return body
	}

	addrBody := tfjson.Body{"name": l.physical(63, n)}
	if lb.Internal {
		addrBody["region"] = l.loc
		addrBody["subnetwork"] = l.net.subnet
		addrBody["address_type"] = "INTERNAL"
		// Several forwarding rules (one per port) share the address.
		addrBody["purpose"] = "SHARED_LOADBALANCER_VIP"
	}
	address := l.cfg.Resource(t.address, n, addrBody)

	var services []string
	for _, rt := range lb.Routes {
		if !slices.Contains(services, rt.Service) {
			services = append(services, rt.Service)
		}
	}
	backends := map[string]string{}
	for _, name := range services {
		if _, ok := l.p.Resource(name).(*ir.Service); !ok {
			continue
		}
		neg := l.cfg.Resource("google_compute_region_network_endpoint_group", n+"_"+id(name), tfjson.Body{
			"name":                  l.physical(63, n, id(name)),
			"region":                l.loc,
			"network_endpoint_type": "SERVERLESS",
			"cloud_run":             tfjson.Body{"service": ref("google_cloud_run_v2_service."+id(name), "name")},
		})
		backend := tfjson.Body{"group": ref(neg, "id")}
		if t.regional {
			backend["balancing_mode"] = "UTILIZATION"
			backend["capacity_scaler"] = 1.0
		}
		backends[name] = l.cfg.Resource(t.backend, n+"_"+id(name), regional(tfjson.Body{
			"name":                  l.physical(63, n, id(name)),
			"load_balancing_scheme": t.scheme,
			"protocol":              "HTTP",
			"backend":               backend,
		}))
	}

	for _, name := range services {
		if addr, ok := backends[name]; ok {
			l.backends[lb.Name] = append(l.backends[lb.Name], addr)
		}
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
	def := root
	if def == nil && len(rules) > 0 {
		// A URL map must send somewhere; ALBs on AWS answer 404 instead.
		def = &lb.Routes[0]
		l.diags.Add(diag.Infof(lb.Pos, "on gcp: %s has no `send /` route, so paths no route matches go to %s", lb.Name, def.Service))
	}
	urlMap := regional(tfjson.Body{"name": l.physical(63, n)})
	if def != nil {
		urlMap["default_service"] = ref(backends[def.Service], "id")
	}
	if len(rules) > 0 {
		var routeRules []tfjson.Body
		for i, rt := range rules {
			routeRules = append(routeRules, tfjson.Body{
				"priority": i + 1,
				// prefix_match is a plain prefix, like the path patterns
				// on AWS: `/admin` matches `/admin/users` and
				// `/administrator`.
				"match_rules": tfjson.Body{"prefix_match": rt.Path},
				"service":     ref(backends[rt.Service], "id"),
			})
		}
		urlMap["host_rule"] = tfjson.Body{"hosts": []string{"*"}, "path_matcher": "routes"}
		urlMap["path_matcher"] = tfjson.Body{
			"name":            "routes",
			"default_service": urlMap["default_service"],
			"route_rules":     routeRules,
		}
	}
	urlMapAddr := l.cfg.Resource(t.urlMap, n, urlMap)

	certs := map[string]string{}
	for _, ln := range lb.Listeners {
		name := fmt.Sprintf("%s_%d", n, ln.Port)
		proxy := regional(tfjson.Body{"name": l.physical(63, n, fmt.Sprint(ln.Port)), "url_map": ref(urlMapAddr, "id")})
		typ := t.httpProxy
		if ln.TLS() {
			cert, ok := certs[ln.Domain]
			if !ok {
				cert = l.certificate(n, ln.Domain, t.regional)
				certs[ln.Domain] = cert
			}
			typ = t.httpsProxy
			proxy["certificate_manager_certificates"] = []tfjson.Expr{ref(cert, "id")}
		}
		proxyAddr := l.cfg.Resource(typ, name, proxy)
		rule := regional(tfjson.Body{
			"name":                  l.physical(63, n, fmt.Sprint(ln.Port)),
			"target":                ref(proxyAddr, "id"),
			"ip_address":            ref(address, "id"),
			"ip_protocol":           "TCP",
			"port_range":            fmt.Sprint(ln.Port),
			"load_balancing_scheme": t.scheme,
		})
		if lb.Internal {
			rule["network"] = l.net.id
			rule["subnetwork"] = l.net.subnet
			// The proxies the load balancer runs on live in this subnet.
			rule["depends_on"] = []string{l.net.proxy}
		}
		l.cfg.Resource(t.forwardingRule, name, rule)
	}

	l.cfg.Outputs = append(l.cfg.Outputs, tfjson.Output{
		Name:        n + "_ip_address",
		Description: "The IP address of load balancer " + lb.Name + "; point your domains at it with an A record",
		Value:       ref(address, "address"),
	})
	if len(lb.Listeners) > 0 {
		l.cfg.Outputs = append(l.cfg.Outputs, tfjson.Output{
			Name:        n + "_url",
			Description: "Where load balancer " + lb.Name + " serves",
			Value:       url(lb.Listeners[0], ref(address, "address")),
		})
	}
}

// url is where a listener can be reached: its domain for TLS, otherwise
// the load balancer's address.
func url(ln ir.Listener, ip tfjson.Expr) tfjson.Expr {
	if ln.TLS() {
		u := "https://" + tfjson.Escape(ln.Domain)
		if ln.Port != 443 {
			u += fmt.Sprintf(":%d", ln.Port)
		}
		return tfjson.Expr(u)
	}
	u := "http://" + string(ip)
	if ln.Port != 80 {
		u += fmt.Sprintf(":%d", ln.Port)
	}
	return tfjson.Expr(u)
}

// certificate creates a Certificate Manager certificate for domain,
// authorised by DNS, and returns its address. The DNS record to create
// is an output, `<lb>_<domain>_validation`.
func (l *lowerer) certificate(lb, domain string, regional bool) string {
	name := lb + "_" + slug(domain)
	located := func(body tfjson.Body) tfjson.Body {
		if regional {
			body["location"] = l.loc
		}
		return body
	}
	// A wildcard is authorised by its parent domain.
	auth := located(tfjson.Body{
		"name":   l.physical(63, lb, slug(domain)),
		"domain": strings.TrimPrefix(domain, "*."),
	})
	if regional {
		auth["type"] = "PER_PROJECT_RECORD"
	}
	authAddr := l.cfg.Resource("google_certificate_manager_dns_authorization", name, auth)
	cert := l.cfg.Resource("google_certificate_manager_certificate", name, located(tfjson.Body{
		"name": l.physical(63, lb, slug(domain)),
		"managed": tfjson.Body{
			"domains":            []string{domain},
			"dns_authorizations": []tfjson.Expr{ref(authAddr, "id")},
		},
		// A replacement must exist before the proxy lets go of the old one.
		"lifecycle": tfjson.Body{"create_before_destroy": true},
	}))
	record := authAddr + ".dns_resource_record[0]"
	l.cfg.Outputs = append(l.cfg.Outputs, tfjson.Output{
		Name:        name + "_validation",
		Description: "The DNS record that proves you own " + domain,
		Value:       tfjson.Expr(fmt.Sprintf("${%s.type} ${%s.name} → ${%s.data}", record, record, record)),
	})
	return cert
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
