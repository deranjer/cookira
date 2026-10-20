package oauth

import "testing"

func TestValidateRedirectURI(t *testing.T) {
	ok := []string{"https://chatgpt.com/connector/oauth/abc", "http://localhost:6274/cb", "http://127.0.0.1/cb", "claude://callback"}
	bad := []string{"http://evil.example/cb", "javascript:alert(1)", "https://u:p@x.example/cb", "https://x.example/cb#frag", "/relative", "https:///nohost"}
	for _, u := range ok {
		if err := validateRedirectURI(u); err != nil {
			t.Errorf("%q rejected: %v", u, err)
		}
	}
	for _, u := range bad {
		if err := validateRedirectURI(u); err == nil {
			t.Errorf("%q accepted", u)
		}
	}
}

func TestRedirectAllowed(t *testing.T) {
	reg := []string{"https://app.example/cb", "http://localhost:1111/cb"}
	cases := map[string]bool{
		"https://app.example/cb":      true,
		"https://app.example/cb2":     false,
		"https://app.example:444/cb":  false,
		"http://localhost:2222/cb":    true, // loopback ports are free
		"http://localhost:2222/other": false,
		"http://127.0.0.1:1111/cb":    false,
		"https://localhost:2222/cb":   false,
		"https://app.example/cb?x=1":  false,
	}
	for uri, want := range cases {
		if got := redirectAllowed(reg, uri); got != want {
			t.Errorf("redirectAllowed(%q) = %v, want %v", uri, got, want)
		}
	}
}
