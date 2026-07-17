package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ygrebnov/model"

	"github.com/ygrebnov/config"
	"github.com/ygrebnov/config/pkg/errors"
	pathPkg "github.com/ygrebnov/config/pkg/path"
)

func getControllerValue[T any](
	t *testing.T,
	controller *config.Controller[T],
	name string,
) any {
	t.Helper()

	value, err := controller.Get(name)
	if err != nil {
		t.Fatalf("Get(%q) error: %v", name, err)
	}

	return value
}

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

func TestNewControllerCtx_RejectsNilContext(t *testing.T) {
	if _, err := config.NewControllerCtx[smallCfg](nil); !errors.Is(err, errors.ErrNilContext) {
		t.Fatalf("NewControllerCtx(nil) error = %v, want ErrNilContext", err)
	}
}

func TestNewControllerCtx_RejectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := config.NewControllerCtx[smallCfg](ctx); err != context.Canceled {
		t.Fatalf("NewControllerCtx(cancelled context) error = %v, want %v", err, context.Canceled)
	}
}

func TestNewController_ReturnsParseError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, path, "name: [\n")

	if _, err := config.NewController[smallCfg](config.WithPath(path)); !errors.Is(err, errors.ErrParse) {
		t.Fatalf("NewController() error = %v, want ErrParse", err)
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

	name := getControllerValue(t, controller, "Name")
	if got := name.(string); got != expected.Name {
		t.Fatalf("unexpected name, got: %v, want: %s", name, expected.Name)
	}

	// update name with an incompatible type
	controller.Set("Name", []int{9}) // type has changed, will not be possible in ControllerTyped[T any]

	name2 := getControllerValue(t, controller, "Name")
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
	name3 := getControllerValue(t, controller, "Name")
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

func TestController_AppNameCreatesAndReloadsConfig(t *testing.T) {
	const appName = "test-app"

	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	first, err := config.NewController[smallCfg](config.WithAppName(appName))
	if err != nil {
		t.Fatalf("first NewController() error: %v", err)
	}

	first.Set("Name", "persisted")
	if err := first.Save(context.Background()); err != nil {
		t.Fatalf("first Save() error: %v", err)
	}

	path := filepath.Join(configHome, appName, pathPkg.DefaultConfigFilename)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file at %q: %v", path, err)
	}

	second, err := config.NewController[smallCfg](config.WithAppName(appName))
	if err != nil {
		t.Fatalf("second NewController() error: %v", err)
	}

	var loaded smallCfg
	if err := second.Load(context.Background(), &loaded); err != nil {
		t.Fatalf("second Load() error: %v", err)
	}
	if loaded.Name != "persisted" {
		t.Fatalf("loaded Name = %q, want %q", loaded.Name, "persisted")
	}
}

func TestController_CustomValidationRuleRejectsInvalidState(t *testing.T) {
	type ruleCfg struct {
		Name string `yaml:"name" default:"allowed" validate:"allowed_name"`
	}

	rule, err := model.NewRule[string](
		"allowed_name",
		func(value string, _ ...string) error {
			if value != "allowed" {
				return fmt.Errorf("name %q is not allowed", value)
			}

			return nil
		},
	)
	if err != nil {
		t.Fatalf("NewRule() error: %v", err)
	}

	path := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, path, "name: allowed\n")

	controller, err := config.NewController[ruleCfg](
		config.WithPath(path),
		config.WithValidationRules(rule),
	)
	if err != nil {
		t.Fatalf("NewController() error: %v", err)
	}

	controller.Set("Name", "invalid")

	err = controller.Validate(context.Background())
	if err == nil || !strings.Contains(err.Error(), `name "invalid" is not allowed`) {
		t.Fatalf("expected custom rule validation error, got %v", err)
	}

	err = controller.Save(context.Background())
	if err == nil || !strings.Contains(err.Error(), `name "invalid" is not allowed`) {
		t.Fatalf("expected custom rule save error, got %v", err)
	}
	if saved := readFile(t, path); saved != "name: allowed\n" {
		t.Fatalf("Save() wrote invalid configuration: %q", saved)
	}

	var loaded ruleCfg
	err = controller.Load(context.Background(), &loaded)
	if !errors.Is(err, errors.ErrCannotLoadConfigurationIntoProvidedObject) {
		t.Fatalf("expected ErrCannotLoadConfigurationIntoProvidedObject, got %v", err)
	}
	if !strings.Contains(err.Error(), `name "invalid" is not allowed`) {
		t.Fatalf("expected custom rule error, got %v", err)
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

	if value := getControllerValue(t, controller, "Name"); value != "service" {
		t.Fatalf("Name = %v, want service", value)
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

	name := getControllerValue(t, controller, "Name")
	if name != "fromdefault" {
		t.Fatalf("expected defaulted name=fromdefault, got %v", name)
	}

	count := getControllerValue(t, controller, "Count")
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

	name := getControllerValue(t, controller, "Name")
	if name != "fromdefault" {
		t.Fatalf("expected defaulted name=fromdefault, got %v", name)
	}

	count := getControllerValue(t, controller, "Count")
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

	name := getControllerValue(t, controller, "Name")
	if name != "fromdefault" {
		t.Fatalf("expected defaulted name=fromdefault, got %v", name)
	}

	count := getControllerValue(t, controller, "Count")
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

	value := getControllerValue(t, controller, "DB.Host")
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

	value := getControllerValue(t, controller, "Name")
	if value != nil {
		t.Fatalf("expected name to be nil, got %v", value)
	}

	if _, err := controller.Get("missing"); !errors.Is(err, errors.ErrConfigurationOptionNotFound) {
		t.Fatalf("expected ErrConfigurationOptionNotFound, got %v", err)
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
	dbHost := getControllerValue(t, controller, "DB.Host")
	if dbHost != "localhost" {
		t.Fatalf("nested field not set: %+v", cfg)
	}
	value := getControllerValue(t, controller, "DB.Host")
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
		_ = getControllerValue(t, controller, name)
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

func TestController_OptionNotFound(t *testing.T) {
	controller, err := config.NewController[smallCfg]()
	if err != nil {
		t.Fatalf("Controller constructor error: %v", err)
	}

	cfg := smallCfg{}
	if err := controller.Load(context.Background(), &cfg); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if _, err := controller.Get("missing"); !errors.Is(err, errors.ErrConfigurationOptionNotFound) {
		t.Fatalf("expected ErrConfigurationOptionNotFound, got %v", err)
	}
}

func TestController_Concurrent_LoadGetSet(t *testing.T) {
	controller, err := config.NewController[smallCfg]()
	if err != nil {
		t.Fatalf("Controller constructor error: %v", err)
	}

	const workers = 16
	start := make(chan struct{})
	errs := make(chan error, workers)

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			<-start

			var cfg smallCfg
			if err := controller.Load(context.Background(), &cfg); err != nil {
				errs <- fmt.Errorf("Load(): %w", err)
				return
			}
			if _, err := controller.Get("Name"); err != nil {
				errs <- fmt.Errorf("Get(Name): %w", err)
				return
			}
			controller.Set("Count", i)

			value, err := controller.Get("Count")
			if err != nil {
				errs <- fmt.Errorf("Get(Count): %w", err)
				return
			}
			if _, ok := value.(int); !ok {
				errs <- fmt.Errorf("Get(Count) value type = %T, want int", value)
			}
		}(i)
	}

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}

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
