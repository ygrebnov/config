package tests

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ygrebnov/config"
)

type ownedItem struct {
	Name string              `yaml:"name"`
	Meta map[string][]string `yaml:"meta"`
}

type ownedConfig struct {
	Items   []ownedItem `yaml:"items"`
	Primary *ownedItem  `yaml:"primary"`
}

func TestControllerMutableValuesAreIndependent(t *testing.T) {
	controller, err := config.NewController[ownedConfig]()
	if err != nil {
		t.Fatal(err)
	}

	items := []ownedItem{{Name: "original", Meta: map[string][]string{"roles": {"reader"}}}}
	primary := &ownedItem{Name: "primary", Meta: map[string][]string{"roles": {"writer"}}}
	controller.Set("Items[]", items)
	controller.Set("Primary", primary)

	items[0].Name = "changed after Set"
	items[0].Meta["roles"][0] = "changed after Set"
	primary.Name = "changed after Set"
	primary.Meta["roles"][0] = "changed after Set"
	assertOwnedValues(t, controller, "original", "reader", "primary", "writer")

	fromGet, err := controller.Get("Items[]")
	if err != nil {
		t.Fatal(err)
	}
	gotItems := fromGet.([]ownedItem)
	gotItems[0].Name = "changed after Get"
	gotItems[0].Meta["roles"][0] = "changed after Get"
	fromGet, err = controller.Get("Primary")
	if err != nil {
		t.Fatal(err)
	}
	gotPrimary := fromGet.(*ownedItem)
	gotPrimary.Name = "changed after Get"
	gotPrimary.Meta["roles"][0] = "changed after Get"
	assertOwnedValues(t, controller, "original", "reader", "primary", "writer")

	var loaded ownedConfig
	if err := controller.Load(context.Background(), &loaded); err != nil {
		t.Fatal(err)
	}
	loaded.Items[0].Name = "changed after Load"
	loaded.Items[0].Meta["roles"][0] = "changed after Load"
	loaded.Primary.Name = "changed after Load"
	loaded.Primary.Meta["roles"][0] = "changed after Load"
	assertOwnedValues(t, controller, "original", "reader", "primary", "writer")

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := controller.Save(context.Background(), config.WithSavePath(path)); err != nil {
		t.Fatal(err)
	}
	saved := readFile(t, path)
	if strings.Contains(saved, "changed after") ||
		!strings.Contains(saved, "original") ||
		!strings.Contains(saved, "writer") {
		t.Fatalf("saved configuration reflects external mutation: %q", saved)
	}
}

func assertOwnedValues(
	t *testing.T,
	controller *config.Controller[ownedConfig],
	itemName, itemRole, primaryName, primaryRole string,
) {
	t.Helper()
	itemsValue, err := controller.Get("Items[]")
	if err != nil {
		t.Fatal(err)
	}
	items := itemsValue.([]ownedItem)
	if len(items) != 1 || items[0].Name != itemName || items[0].Meta["roles"][0] != itemRole {
		t.Fatalf("stored items changed: %+v", items)
	}
	primaryValue, err := controller.Get("Primary")
	if err != nil {
		t.Fatal(err)
	}
	primary := primaryValue.(*ownedItem)
	if primary.Name != primaryName || primary.Meta["roles"][0] != primaryRole {
		t.Fatalf("stored primary changed: %+v", primary)
	}
}
