package websocket

import (
	"errors"
	"testing"

	"github.com/cucumber/godog"

	"github.com/tomatool/tomato/internal/jsonmatch"
)

func TestLastMessageComparisonsFailWithAMismatch(t *testing.T) {
	r := &WebSocketClient{lastMessage: []byte(`{"b":2,"a":1}`)}

	err := r.lastMessageShouldBe(&godog.DocString{Content: `{"a":1}`})
	if want := "message mismatch:\nexpected: {\"a\":1}\nactual: {\"b\":2,\"a\":1}"; err == nil || err.Error() != want {
		t.Fatalf("text err = %v, want %q", err, want)
	}
	var m *jsonmatch.Mismatch
	if !errors.As(err, &m) || m.Mode != jsonmatch.ModeText {
		t.Errorf("text mismatch = %+v", m)
	}

	err = r.lastMessageShouldBeJSONMatching(&godog.DocString{Content: `{"a": 1}`})
	if want := "JSON mismatch:\nexpected: {\"a\":1}\nactual: {\"a\":1,\"b\":2}"; err == nil || err.Error() != want {
		t.Fatalf("JSON err = %v, want %q", err, want)
	}
	if !errors.As(err, &m) || m.Mode != jsonmatch.ModeJSON || m.What != "JSON" {
		t.Errorf("JSON mismatch = %+v", m)
	}
}
