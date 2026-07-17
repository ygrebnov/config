package tests

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ygrebnov/config"
	"github.com/ygrebnov/config/pkg/errors"
)

func TestController(t *testing.T) {
	tests := []struct {
		name                     string
		opts                     []config.Option
		before                   func(t *testing.T)
		expectedCfg              cfg
		expectedConstructorError error
		expectedLoadError        error
	}{
		{
			name:        "no options, only object itself",
			expectedCfg: cfg{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.before != nil {
				tt.before(t)
			}

			c, err := config.NewController[cfg](tt.opts...)
			if tt.expectedConstructorError != nil {
				if err == nil {
					t.Fatalf("expected NewController to return an error, but got none")
				}
				if !errors.Is(err, tt.expectedConstructorError) {
					t.Fatalf("expected NewController to return error: %v, got %v", tt.expectedConstructorError, err)
				}
			} else if err != nil {
				t.Fatalf("expected NewController to return no error, but got: %v", err)
			}

			obj := cfg{}
			err2 := c.Load(context.Background(), &obj)
			if tt.expectedLoadError != nil {
				if err2 == nil {
					t.Fatalf("expected Load to return an error, but got none")
				}
				if !errors.Is(err2, tt.expectedLoadError) {
					t.Fatalf("expected Load to return error: %v, got %v", tt.expectedLoadError, err2)
				}
			} else if err2 != nil {
				t.Fatalf("expected Load to return no error, but got: %v", err2)
			}

			checkCfg(t, obj, tt.expectedCfg)
		})
	}
}

func TestController_LoadGetSetSave(t *testing.T) {
	td := t.TempDir()
	path := filepath.Join(td, "config.yaml")
	writeFile(t, path, "name: fromfile\nport: 2\n")

	controller, err := config.NewController[cfg](config.WithPath(path))
	if err != nil {
		t.Fatalf("Controller constructor error: %v", err)
	}

	expected := cfg{Name: "fromfile", Port: 2}
	var obj cfg
	if err = controller.Load(context.Background(), &obj); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	checkCfg(t, obj, expected)

	name, ok := controller.Get("Name")
	if !ok {
		t.Fatalf("expected Name setting to be present")
	}
	if got := name.(string); got != expected.Name {
		t.Fatalf("unexpected name, got: %v, want: %s", name, expected.Name)
	}

	// update name with an incompatible type
	controller.Set("Name", []int{9}) // type has changed, will not be possible in ControllerTyped[T any]

	name2, ok2 := controller.Get("Name")
	if !ok2 {
		t.Fatalf("expected Name setting to be present")
	}
	if got := name2.([]int); len(got) != 1 || got[0] != 9 {
		t.Fatalf("unexpected name, got: %v, want: [9]", name2)
	}

	var obj2 cfg
	if err2 := controller.Load(context.Background(), &obj2); err2 == nil {
		t.Fatalf("expected Load() to return an error, but got none")
	} else if !errors.Is(err2, errors.ErrCannotLoadConfigurationIntoProvidedObject) {
		t.Fatalf(
			"expected Load() to return error: %v, got: %v",
			errors.ErrCannotLoadConfigurationIntoProvidedObject,
			err2,
		)
	}

	// update name with changing type back to string
	controller.Set("Name", "newname")
	name3, ok3 := controller.Get("Name")
	if !ok3 {
		t.Fatalf("expected Name setting to be present")
	}
	if got := name3.(string); got != "newname" {
		t.Fatalf("unexpected name, got: %v, want: %s", name3, "newname")
	}

	expected3 := cfg{Name: "newname", Port: 2}
	var obj3 cfg
	if err3 := controller.Load(context.Background(), &obj3); err3 != nil {
		t.Fatalf("Load() error: %v", err3)
	}
	checkCfg(t, obj3, expected3)

	// the file on disk contains initial data, because Save has not been called.
	onDisk := readFile(t, path)
	expectedOnDisk := "name: fromfile\nport: 2\n"
	if onDisk != expectedOnDisk {
		t.Fatalf("unexpected onDisk, got: %s, want: %s", onDisk, expectedOnDisk)
	}

	if err = controller.Save(context.Background()); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// the file on disk is updated now with the normalized configuration.
	onDisk2 := readFile(t, path)
	if !strings.Contains(onDisk2, "name: newname\n") ||
		!strings.Contains(onDisk2, "port: 2\n") ||
		strings.Contains(onDisk2, "name: fromfile\n") {
		t.Fatalf("unexpected normalized configuration: %s", onDisk2)
	}
}

func TestController_TranslatesTaggedJSONPaths(t *testing.T) {
	td := t.TempDir()
	path := filepath.Join(td, "config.json")
	writeFile(t, path, `{"name":"service","count":3,"dur":"1s"}`)

	controller, err := config.NewController[smallCfg](config.WithPath(path))
	if err != nil {
		t.Fatalf("NewController() error: %v", err)
	}

	if value, found := controller.Get("Name"); !found || value != "service" {
		t.Fatalf("Name = %v, %t; want service, true", value, found)
	}

	controller.Set("Name", "saved")
	if err := controller.Save(context.Background()); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	if saved := readFile(t, path); saved != `{"name":"saved","count":3,"dur":"1s"}` {
		t.Fatalf("saved JSON = %s", saved)
	}
}

func TestNewController_NormalizesDefaultsIntoStore(t *testing.T) {
	type defaultCfg struct {
		Name  string `yaml:"name" default:"fromdefault"`
		Count int    `yaml:"count" default:"7"`
	}

	controller, err := config.NewController[defaultCfg]()
	if err != nil {
		t.Fatalf("Controller constructor error: %v", err)
	}

	name, found := controller.Get("Name")
	if !found {
		t.Fatal("expected defaulted name to be present in store")
	}
	if name != "fromdefault" {
		t.Fatalf("expected defaulted name=fromdefault, got %v", name)
	}

	count, found := controller.Get("Count")
	if !found {
		t.Fatal("expected defaulted count to be present in store")
	}
	if count != 7 {
		t.Fatalf("expected defaulted count=7, got %v", count)
	}

	loaded := defaultCfg{}
	if err := controller.Load(context.Background(), &loaded); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.Name != "fromdefault" || loaded.Count != 7 {
		t.Fatalf("unexpected loaded config: %+v", loaded)
	}
}

func TestNewController_NormalizesDefaultsIntoStore_JSON(t *testing.T) {
	type defaultCfg struct {
		Name  string `json:"name" default:"fromdefault"`
		Count int    `json:"count" default:"7"`
	}

	controller, err := config.NewController[defaultCfg]()
	if err != nil {
		t.Fatalf("Controller constructor error: %v", err)
	}

	name, found := controller.Get("Name")
	if !found {
		t.Fatal("expected defaulted name to be present in store")
	}
	if name != "fromdefault" {
		t.Fatalf("expected defaulted name=fromdefault, got %v", name)
	}

	count, found := controller.Get("Count")
	if !found {
		t.Fatal("expected defaulted count to be present in store")
	}
	if count != 7 {
		t.Fatalf("expected defaulted count=7, got %v", count)
	}

	loaded := defaultCfg{}
	if err := controller.Load(context.Background(), &loaded); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.Name != "fromdefault" || loaded.Count != 7 {
		t.Fatalf("unexpected loaded config: %+v", loaded)
	}
}

func TestNewController_NormalizesDefaultsIntoStore_NoTags(t *testing.T) {
	type defaultCfg struct {
		Name  string `default:"fromdefault"`
		Count int    `default:"7"`
	}

	controller, err := config.NewController[defaultCfg]()
	if err != nil {
		t.Fatalf("Controller constructor error: %v", err)
	}

	name, found := controller.Get("Name")
	if !found {
		t.Fatal("expected defaulted name to be present in store")
	}
	if name != "fromdefault" {
		t.Fatalf("expected defaulted name=fromdefault, got %v", name)
	}

	count, found := controller.Get("Count")
	if !found {
		t.Fatal("expected defaulted count to be present in store")
	}
	if count != 7 {
		t.Fatalf("expected defaulted count=7, got %v", count)
	}

	loaded := defaultCfg{}
	if err := controller.Load(context.Background(), &loaded); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.Name != "fromdefault" || loaded.Count != 7 {
		t.Fatalf("unexpected loaded config: %+v", loaded)
	}
}

func TestNewController_PreservesNestedKeysLoadedFromFile(t *testing.T) {
	type nestedCfg struct {
		DB *struct {
			Host string `yaml:"host"`
		} `yaml:"db"`
	}

	td := t.TempDir()
	path := filepath.Join(td, "config.yaml")
	writeFile(t, path, "db:\n  host: localhost\n")

	controller, err := config.NewController[nestedCfg](config.WithPath(path))
	if err != nil {
		t.Fatalf("Controller constructor error: %v", err)
	}

	value, found := controller.Get("DB.Host")
	if !found {
		t.Fatal("expected nested key DB.Host to be present in store")
	}
	if value != "localhost" {
		t.Fatalf("expected db.host=localhost, got %v", value)
	}

	loaded := nestedCfg{}
	if err := controller.Load(context.Background(), &loaded); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.DB == nil || loaded.DB.Host != "localhost" {
		t.Fatalf("unexpected loaded nested config: %+v", loaded)
	}
}

func TestNewController_NilValueIsStoredAsPresent(t *testing.T) {
	type nullableCfg struct {
		Name *string `yaml:"name"`
	}

	td := t.TempDir()
	path := filepath.Join(td, "config.yaml")
	writeFile(t, path, "name: null\n")

	controller, err := config.NewController[nullableCfg](config.WithPath(path))
	if err != nil {
		t.Fatalf("Controller constructor error: %v", err)
	}

	value, found := controller.Get("Name")
	if !found {
		t.Fatal("expected key with nil value to be present in store")
	}
	if value != nil {
		t.Fatalf("expected name to be nil, got %v", value)
	}

	if _, found := controller.Get("missing"); found {
		t.Fatal("expected missing key to be absent from store")
	}

	loaded := nullableCfg{}
	if err := controller.Load(context.Background(), &loaded); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.Name != nil {
		t.Fatalf("expected loaded name to remain nil, got %v", *loaded.Name)
	}
}

func TestController_GetSet_NestedOption(t *testing.T) {
	type nestedCfg struct {
		DB *struct {
			Host string `yaml:"host"`
		} `yaml:"db"`
	}

	controller, err := config.NewController[nestedCfg]()
	if err != nil {
		t.Fatalf("Controller constructor error: %v", err)
	}

	cfg := nestedCfg{}
	if err := controller.Load(context.Background(), &cfg); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	controller.Set("DB.Host", "localhost")
	dbHost, _ := controller.Get("DB.Host")
	if dbHost != "localhost" {
		t.Fatalf("nested field not set: %+v", cfg)
	}
	value, _ := controller.Get("DB.Host")
	if value != "localhost" {
		t.Fatalf("unexpected nested value: %v", value)
	}
}

func TestController_TranslatesTaggedNestedCollectionPaths(t *testing.T) {
	type item struct {
		Name string `yaml:"name"`
	}
	type taggedCfg struct {
		Name   string `yaml:"name"`
		Server struct {
			Host string `yaml:"host"`
		} `yaml:"server"`
		Items []item `yaml:"items"`
	}

	td := t.TempDir()
	path := filepath.Join(td, "config.yaml")
	writeFile(t, path, "name: service\nserver:\n  host: localhost\nitems:\n  - name: first\n")

	controller, err := config.NewController[taggedCfg](config.WithPath(path))
	if err != nil {
		t.Fatalf("NewController() error: %v", err)
	}

	for _, name := range []string{"Name", "Server.Host", "Items[]"} {
		if _, found := controller.Get(name); !found {
			t.Fatalf("expected model path %q to be present", name)
		}
	}

	var loaded taggedCfg
	if err := controller.Load(context.Background(), &loaded); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.Name != "service" || loaded.Server.Host != "localhost" ||
		len(loaded.Items) != 1 || loaded.Items[0].Name != "first" {
		t.Fatalf("unexpected loaded configuration: %+v", loaded)
	}

	controller.Set("Server.Host", "saved")
	if err := controller.Save(context.Background()); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	saved := readFile(t, path)
	for _, fragment := range []string{
		"name: service",
		"server:\n    host: saved",
		"items:\n    - name: first",
	} {
		if !strings.Contains(saved, fragment) {
			t.Fatalf("saved configuration %q does not contain %q", saved, fragment)
		}
	}
	if strings.Contains(saved, "Name:") ||
		strings.Contains(saved, "Server:") ||
		strings.Contains(saved, "Items[]:") {
		t.Fatalf("saved configuration used model paths: %q", saved)
	}
}

func TestController_PreservesYAMLOnlyAndInlineFields(t *testing.T) {
	type inlineCfg struct {
		Host string `yaml:"host"`
	}
	type taggedCfg struct {
		Secret string    `yaml:"secret" json:"-"`
		Inline inlineCfg `yaml:",inline"`
	}

	td := t.TempDir()
	path := filepath.Join(td, "config.yaml")
	writeFile(t, path, "secret: keep\nhost: localhost\n")

	controller, err := config.NewController[taggedCfg](config.WithPath(path))
	if err != nil {
		t.Fatalf("NewController() error: %v", err)
	}

	var loaded taggedCfg
	if err := controller.Load(context.Background(), &loaded); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.Secret != "keep" || loaded.Inline.Host != "localhost" {
		t.Fatalf("unexpected loaded configuration: %+v", loaded)
	}

	controller.Set("Inline.Host", "saved")
	if err := controller.Save(context.Background()); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	saved := readFile(t, path)
	if !strings.Contains(saved, "secret: keep\n") ||
		!strings.Contains(saved, "host: saved\n") ||
		strings.Contains(saved, "inline:") {
		t.Fatalf("unexpected saved YAML: %q", saved)
	}
}

func TestController_SaveWithoutPath(t *testing.T) {
	controller, err := config.NewController[smallCfg]()
	if err != nil {
		t.Fatalf("Controller constructor error: %v", err)
	}
	cfg := smallCfg{}
	if err := controller.Load(context.Background(), &cfg); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if err := controller.Save(context.Background()); !errors.Is(err, errors.ErrPathNotConfigured) {
		t.Fatalf("expected ErrPathNotConfigured, got %v", err)
	}
}

func TestController_OptionNotFound(t *testing.T) {
	controller, err := config.NewController[smallCfg]()
	if err != nil {
		t.Fatalf("Controller constructor error: %v", err)
	}

	cfg := smallCfg{}
	if err := controller.Load(context.Background(), &cfg); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if _, found := controller.Get("missing"); found {
		t.Fatalf("expected Get() to not find 'missing', but it did")
	}
}

/*
func TestController_Concurrent_LoadGetSet(t *testing.T) {
	controller := NewController[testCfg2]()
	cfg := testCfg2{Name: "default"}

	const workers = 16
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := controller.Load(nil, &cfg); err != nil {
				t.Errorf("Load() error: %v", err)
				return
			}
			if _, err := controller.Get("name"); err != nil {
				t.Errorf("Get() error: %v", err)
			}
			if err := controller.Set("count", i); err != nil {
				t.Errorf("Set() error: %v", err)
			}
		}(i)
	}
	wg.Wait()
}

*/

func TestController_SameTarget_DifferentSettingsIsolated(t *testing.T) {
	td := t.TempDir()
	pathA := filepath.Join(td, "a.yaml")
	pathB := filepath.Join(td, "b.yaml")
	writeFile(t, pathA, "name: first\n")
	writeFile(t, pathB, "name: second\n")

	target := smallCfg{}
	controllerA, errA := config.NewController[smallCfg](config.WithPath(pathA))
	if errA != nil {
		t.Fatalf("ControllerA constructor error: %v", errA)
	}
	controllerB, errB := config.NewController[smallCfg](config.WithPath(pathB))
	if errB != nil {
		t.Fatalf("ControllerB constructor error: %v", errB)
	}

	if err := controllerA.Load(context.Background(), &target); err != nil {
		t.Fatalf("controllerA.Load() error: %v", err)
	}
	if target.Name != "first" {
		t.Fatalf("unexpected target after controllerA load: %+v", target)
	}

	other := smallCfg{}
	if err := controllerB.Load(context.Background(), &other); err != nil {
		t.Fatalf("controllerB.Load() error: %v", err)
	}
	if other.Name != "second" {
		t.Fatalf("unexpected target after controllerB load: %+v", other)
	}
}
