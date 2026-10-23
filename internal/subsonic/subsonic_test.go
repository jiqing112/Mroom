package subsonic

import "testing"

func TestJSONPCallbackValidation(t *testing.T) {
	cases := map[string]string{
		"cb":              "cb",
		"jQuery123_456":   "jQuery123_456",
		"window.callback": "window.callback",
		"$fn":             "$fn",
		"1bad":            "",
		"a-b":             "",
		"alert(1)":        "",
		"</script>":       "",
		"":                "",
	}
	for in, want := range cases {
		if got := jsonpCallback(in); got != want {
			t.Errorf("jsonpCallback(%q) = %q, want %q", in, got, want)
		}
	}
}
