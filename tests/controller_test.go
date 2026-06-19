package tests

import (
	"context"
	"path/filepath"
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

	name, ok := controller.Get("name")
	if !ok {
		t.Fatalf("expected name setting to be present")
	}
	if got := name.(string); got != expected.Name {
		t.Fatalf("unexpected name, got: %v, want: %s", name, expected.Name)
	}

	// update name with type change
	controller.Set("name", 9) // type has changed, will not be possible in ControllerTyped[T any]

	name2, ok2 := controller.Get("name")
	if !ok2 {
		t.Fatalf("expected name setting to be present")
	}
	if got := name2.(int); got != 9 {
		t.Fatalf("unexpected name, got: %v, want: %d", name2, 9)
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
	controller.Set("name", "newname")
	name3, ok3 := controller.Get("name")
	if !ok3 {
		t.Fatalf("expected name setting to be present")
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

	// the file on disk is updated now.
	onDisk2 := readFile(t, path)
	expectedOnDisk2 := "name: newname\nport: 2\n"
	if onDisk2 != expectedOnDisk2 {
		t.Fatalf("unexpected onDisk, got: %s, want: %s", onDisk2, expectedOnDisk2)
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

	name, found := controller.Get("name")
	if !found {
		t.Fatal("expected defaulted name to be present in store")
	}
	if name != "fromdefault" {
		t.Fatalf("expected defaulted name=fromdefault, got %v", name)
	}

	count, found := controller.Get("count")
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

	name, found := controller.Get("name")
	if !found {
		t.Fatal("expected defaulted name to be present in store")
	}
	if name != "fromdefault" {
		t.Fatalf("expected defaulted name=fromdefault, got %v", name)
	}

	count, found := controller.Get("count")
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

	name, found := controller.Get("name")
	if !found {
		t.Fatal("expected defaulted name to be present in store")
	}
	if name != "fromdefault" {
		t.Fatalf("expected defaulted name=fromdefault, got %v", name)
	}

	count, found := controller.Get("count")
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

	value, found := controller.Get("db.host")
	if !found {
		t.Fatal("expected nested key db.host to be present in store")
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

	value, found := controller.Get("name")
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
	controller.Set("db.host", "localhost")
	dbHost, _ := controller.Get("db.host")
	if dbHost != "localhost" {
		t.Fatalf("nested field not set: %+v", cfg)
	}
	value, _ := controller.Get("db.host")
	if value != "localhost" {
		t.Fatalf("unexpected nested value: %v", value)
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
