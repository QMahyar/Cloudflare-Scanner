package main

import (
	"reflect"
	"testing"
)

func TestParseVer(t *testing.T) {
	cases := []struct {
		in   string
		want []int
	}{
		{"v3.7.0", []int{3, 7, 0}},
		{"3.7.0", []int{3, 7, 0}},
		{"V3.7.0", []int{3, 7, 0}},
		{"3.7.0-rc1", []int{3, 7, 0}},
		{"3.7.0+build", []int{3, 7, 0}},
		{"  v1.2.3  ", []int{1, 2, 3}},
		{"dev", nil},
		{"", nil},
		{"   ", nil},
		{"not-a-version", nil},
		{"1.x.0", nil},
	}
	for _, c := range cases {
		got := parseVer(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("parseVer(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v3.6.0", "v3.7.0", true},
		{"v3.7.0", "v3.7.0", false},
		{"v3.7.0", "v3.6.9", false},
		{"3.7.0", "3.7.1", true},
		{"v3.7.0-rc1", "v3.7.0", false}, // same numeric triple after strip
		{"dev", "v1.0.0", true},
		{"", "v1.0.0", true},
		{"v1.0.0", "dev", false},
		{"v1.0.0", "", false},
		{"v1.0.0", "garbage", false},
		{"dev", "", false}, // nil current, empty latest → false
		{"1.2", "1.2.0", false},
		{"1.2", "1.2.1", true},
		{"2.0.0", "10.0.0", true},
	}
	for _, c := range cases {
		if got := isNewerVersion(c.current, c.latest); got != c.want {
			t.Errorf("isNewerVersion(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}
