package youtubedl

import (
	"encoding/base64"
	"net/url"
	"testing"

	"github.com/dop251/goja/parser"
)

func TestPlayer(t *testing.T) {
	p, err := NewPlayer()
	if err != nil {
		t.Fatalf("failed to create player: %v", err)
	}

	if p.sig_timestamp < 0 {
		t.Errorf("p.sig_timestamp is negative")
	}
	if p.visitorData == "" {
		t.Fatalf("p.visitorData is empty")
	}

	visitorData, err := url.QueryUnescape(p.visitorData)
	if err != nil {
		t.Errorf("failed to unescape visitorData: %v", err)
	}
	_, err = base64.URLEncoding.DecodeString(visitorData)
	if err != nil {
		t.Errorf("failed to decode visitorData: %v", err)
	}

	if p.nsig_sc != "" {
		if p.nsig_name == "" {
			t.Errorf("p.nsig_name is empty while p.nsig_sc is set")
		}

		_, err = parser.ParseFile(nil, "", p.nsig_sc, 0)
		if err != nil {
			t.Errorf("failed to parse p.nsig_sc: %v", err)
		}
	}

	if p.sig_sc != "" {
		_, err = parser.ParseFile(nil, "", p.sig_sc, 0)
		if err != nil {
			t.Errorf("failed to parse p.sig_sc: %v", err)
		}
	}
}
