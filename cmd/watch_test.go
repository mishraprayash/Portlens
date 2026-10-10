package cmd

import (
	"reflect"
	"testing"
)

func TestDiffWatch(t *testing.T) {
	prev := watchSnap{
		"Port 3000": "up:100:node",
		"Port 4000": "down",
	}
	cur := watchSnap{
		"Port 3000": "up:100:node",
		"Port 4000": "up:200:python",
		"Port 5000": "up:300:go",
	}
	got := diffWatch(prev, cur)
	want := []watchChange{
		{kind: "up", target: "Port 4000", detail: "up:200:python"},
		{kind: "up", target: "Port 5000", detail: "up:300:go"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("diffWatch = %+v, want %+v", got, want)
	}
}

func TestDiffWatchDown(t *testing.T) {
	prev := watchSnap{"Port 3000": "up:1:node"}
	got := diffWatch(prev, watchSnap{"Port 3000": "down"})
	want := []watchChange{{kind: "down", target: "Port 3000"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("diffWatch = %+v, want %+v", got, want)
	}
}

func TestDiffWatchProcessChange(t *testing.T) {
	prev := watchSnap{"Port 3000": "up:1:node"}
	got := diffWatch(prev, watchSnap{"Port 3000": "up:2:node"})
	want := []watchChange{{kind: "changed", target: "Port 3000", detail: "up:2:node"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("diffWatch = %+v, want %+v", got, want)
	}
}
