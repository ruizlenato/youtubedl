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

	if p.sigTimestamp < 0 {
		t.Errorf("p.sigTimestamp is negative")
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

	if p.nsigSC != "" {
		if p.nsigName == "" {
			t.Errorf("p.nsigName is empty while p.nsigSC is set")
		}

		_, err = parser.ParseFile(nil, "", p.nsigSC, 0)
		if err != nil {
			t.Errorf("failed to parse p.nsigSC: %v", err)
		}
	}

	if p.sigSC != "" {
		_, err = parser.ParseFile(nil, "", p.sigSC, 0)
		if err != nil {
			t.Errorf("failed to parse p.sigSC: %v", err)
		}
	}
}
