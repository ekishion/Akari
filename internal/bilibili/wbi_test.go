package bilibili

import (
	"testing"
)

func TestWbiSigner_MixinKey(t *testing.T) {
	// Example test keys from Bilibili WBI docs
	imgKey := "7cd084941338484aae1ad9425b84077c"
	subKey := "4932c490c3b539b7ac82435186d235c0"

	mixinKey := CalculateMixinKey(imgKey, subKey)
	if len(mixinKey) != 32 {
		t.Fatalf("expected 32 char mixinKey, got %d (%s)", len(mixinKey), mixinKey)
	}

	signer := NewWbiSigner()
	signer.UpdateKeys(imgKey, subKey)

	params := map[string]string{
		"foo": "114",
		"bar": "514",
		"wts": "1684085779",
	}

	signed := signer.SignParams(params)
	if signed["w_rid"] == "" {
		t.Fatalf("expected w_rid to be generated")
	}
	if signed["wts"] != "1684085779" {
		t.Fatalf("expected wts to be preserved, got %s", signed["wts"])
	}
}

func TestWbiSigner_ExtractKey(t *testing.T) {
	url1 := "https://i0.hdslb.com/bfs/wbi/7cd084941338484aae1ad9425b84077c.png"
	url2 := "https://i0.hdslb.com/bfs/wbi/4932c490c3b539b7ac82435186d235c0.png"

	k1 := extractKeyFromURL(url1)
	k2 := extractKeyFromURL(url2)

	if k1 != "7cd084941338484aae1ad9425b84077c" {
		t.Errorf("unexpected k1: %s", k1)
	}
	if k2 != "4932c490c3b539b7ac82435186d235c0" {
		t.Errorf("unexpected k2: %s", k2)
	}

	signer := NewWbiSigner()
	signer.UpdateFromURLs(url1, url2)
	if !signer.HasKeys() {
		t.Errorf("expected signer to have keys")
	}
}
