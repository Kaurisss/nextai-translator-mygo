package app

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

func TestCredentialStoreRoundTrip(t *testing.T) {
	if os.Getenv("NEXTAI_DESKTOP_INTEGRATION") != "1" {
		t.Skip("set NEXTAI_DESKTOP_INTEGRATION=1 for native credential store test")
	}
	store := mygo.Secrets.Service("xyz.nextai.translator.native.tests")
	key := fmt.Sprint("roundtrip-", time.Now().UnixNano())
	if err := store.Set(key, []byte("native-test-value")); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Delete(key); err != nil {
			t.Error(err)
		}
	}()
	got, err := store.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "native-test-value" {
		t.Fatal("credential value did not round trip")
	}
}
