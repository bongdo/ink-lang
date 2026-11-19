package main

import "testing"

func TestParseRunArgs(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		race    bool
		rest    []string
		wantErr bool
	}{
		{name: "none"},
		{name: "race", args: []string{"-race", "a", "b"}, race: true, rest: []string{"a", "b"}},
		{name: "race separated", args: []string{"--race", "--", "-x"}, race: true, rest: []string{"-x"}},
		{name: "separator", args: []string{"--", "-race"}, rest: []string{"-race"}},
		{name: "program args", args: []string{"a", "--race"}, rest: []string{"a", "--race"}},
		{name: "bad flag", args: []string{"--nope"}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			race, rest, err := parseRunArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if race != tc.race {
				t.Fatalf("race = %v, want %v", race, tc.race)
			}
			if len(rest) != len(tc.rest) {
				t.Fatalf("rest = %v, want %v", rest, tc.rest)
			}
			for i := range rest {
				if rest[i] != tc.rest[i] {
					t.Fatalf("rest = %v, want %v", rest, tc.rest)
				}
			}
		})
	}
}

func TestDirArgBeforeSeparator(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{name: "dot", args: []string{"."}, want: true},
		{name: "after separator", args: []string{"--", "."}},
		{name: "with race", args: []string{"-race", "."}, want: true},
		{name: "temp dir", args: []string{dir}, want: true},
		{name: "plain arg", args: []string{"not-a-dir"}},
		{name: "no args"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, got := dirArgBeforeSeparator(tc.args)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
