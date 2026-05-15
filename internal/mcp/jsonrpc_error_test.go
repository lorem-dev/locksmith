package mcp

import "testing"

func TestInspectResponse(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{
			"tool isError true",
			`{"jsonrpc":"2.0","id":1,"result":{"isError":true,"content":[{"type":"text","text":"x"}]}}`,
			true,
		},
		{
			"tool isError false",
			`{"jsonrpc":"2.0","id":1,"result":{"isError":false}}`,
			false,
		},
		{
			"tool result no isError",
			`{"jsonrpc":"2.0","id":1,"result":{"content":[]}}`,
			false,
		},
		{
			"jsonrpc error object",
			`{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"bad"}}`,
			true,
		},
		{
			"notification (no id)",
			`{"jsonrpc":"2.0","method":"notify","params":{}}`,
			false,
		},
		{
			"id null",
			`{"jsonrpc":"2.0","id":null,"error":{"code":-1}}`,
			false,
		},
		{
			"malformed json",
			`garbage{not json`,
			false,
		},
		{
			"non-jsonrpc",
			`{"foo":"bar"}`,
			false,
		},
		{
			"wrong jsonrpc version",
			`{"jsonrpc":"1.0","id":1,"error":{}}`,
			false,
		},
		{
			"string id with isError",
			`{"jsonrpc":"2.0","id":"abc","result":{"isError":true}}`,
			true,
		},
		{
			"empty result object",
			`{"jsonrpc":"2.0","id":1,"result":{}}`,
			false,
		},
		{
			"both error and result (error wins)",
			`{"jsonrpc":"2.0","id":1,"error":{"code":-1},"result":{}}`,
			true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := inspectResponse([]byte(tc.raw))
			if got != tc.want {
				t.Errorf("inspectResponse(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestExtractID(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			"numeric id",
			`{"jsonrpc":"2.0","id":1,"result":{}}`,
			"1",
		},
		{
			"string id",
			`{"jsonrpc":"2.0","id":"abc","result":{}}`,
			`"abc"`,
		},
		{
			"id null",
			`{"jsonrpc":"2.0","id":null,"result":{}}`,
			"",
		},
		{
			"missing id",
			`{"jsonrpc":"2.0","method":"notify"}`,
			"",
		},
		{
			"malformed",
			`garbage`,
			"",
		},
		{
			"id with spaces inside",
			`{"jsonrpc":"2.0","id":42,"result":{}}`,
			"42",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractID([]byte(tc.raw))
			if got != tc.want {
				t.Errorf("extractID(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}
