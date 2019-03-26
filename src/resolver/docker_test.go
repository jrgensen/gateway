package resolver

import "testing"

func TestGetDestinationHostPort(t *testing.T) {
	d := &Docker{gatewayIp: "gateway"}
	d.proxyMappings, _ = d.parseProxyMappings("src:bob:80 abc:3000 web.site.com:web") //map[string]string{"bob": "5"} //  "src:dst:80 host:80 web.site.com:web",
	d.portMappings = map[string]uint16{"bob:80": 5, "bob_stack_1:80": 5, "abc_stack:3000": 18, "abc_stack_1:3000": 18, "bob_stack:80": 5, "abc:3000": 18, "web:80": 42}

	dstHostPort, err := d.GetDestinationHostPort("abc.bob.local.test.tld")
	if dstHostPort != "gateway:5" {
		t.Errorf("Source host should equal destination host and have default port, got: %s. (%#v)", dstHostPort, err)
	}

	dstHostPort, err = d.GetDestinationHostPort("abc.abc.local.test.tld")
	if dstHostPort != "gateway:18" {
		t.Errorf("Source host should equal destination host and have default port, got: %s. (%#v)", dstHostPort, err)
	}

	dstHostPort, err = d.GetDestinationHostPort("web.site.com")
	if dstHostPort != "gateway:42" {
		t.Errorf("Source host should equal destination host and have default port, got: %s. (%#v)", dstHostPort, err)
	}
}
