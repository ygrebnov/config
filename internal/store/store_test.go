package store

import "testing"

func TestStoreCopiesCyclicValues(t *testing.T) {
	type node struct {
		Name string
		Next *node
	}
	original := &node{Name: "original"}
	original.Next = original

	store := New()
	store.Set("node", original)
	original.Name = "changed"

	value, ok := store.Get("node")
	if !ok {
		t.Fatal("missing stored value")
	}
	got := value.(*node)
	if got.Name != "original" || got.Next != got {
		t.Fatalf("cyclic value was not copied: %+v", got)
	}
	got.Name = "changed again"
	again, _ := store.Get("node")
	if again.(*node).Name != "original" {
		t.Fatal("Get returned a reference to stored value")
	}
}
