package examples

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ygrebnov/config"
	"github.com/ygrebnov/model"
)

func ExampleLoad() {
	type Config struct {
		Name string `yaml:"name"`
		Port int    `yaml:"port"`
	}

	dir, err := os.MkdirTemp("", "config-example-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(
		path,
		[]byte("name: api\nport: 8080\n"),
		0o600,
	); err != nil {
		panic(err)
	}

	var cfg Config
	if err := config.Load(
		context.Background(),
		&cfg,
		config.WithPath(path),
	); err != nil {
		panic(err)
	}

	fmt.Printf("%s:%d\n", cfg.Name, cfg.Port)

	// Output:
	// api:8080
}

func ExampleController() {
	type Config struct {
		Name string `yaml:"name"`
		Port int    `yaml:"port" validate:"min(1)"`
	}

	dir, err := os.MkdirTemp("", "controller-example-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(
		path,
		[]byte("name: api\nport: 8080\n"),
		0o600,
	); err != nil {
		panic(err)
	}

	controller, err := config.NewController[Config](config.WithPath(path))
	if err != nil {
		panic(err)
	}

	controller.Set("Port", 9090)
	if err := controller.Validate(context.Background()); err != nil {
		panic(err)
	}
	if err := controller.Save(context.Background()); err != nil {
		panic(err)
	}

	var cfg Config
	if err := controller.Load(context.Background(), &cfg); err != nil {
		panic(err)
	}

	name, err := controller.Get("Name")
	if err != nil {
		panic(err)
	}

	fmt.Printf("%s:%d\n", name, cfg.Port)

	// Output:
	// api:9090
}

func ExampleWithValidationRules() {
	type Config struct {
		Environment string `default:"production" validate:"environment"`
	}

	rule, err := model.NewRule[string](
		"environment",
		func(value string, _ ...string) error {
			if value != "production" && value != "development" {
				return fmt.Errorf("unsupported environment %q", value)
			}

			return nil
		},
	)
	if err != nil {
		panic(err)
	}

	controller, err := config.NewController[Config](
		config.WithValidationRules(rule),
	)
	if err != nil {
		panic(err)
	}

	controller.Set("Environment", "staging")
	fmt.Println(controller.Validate(context.Background()) != nil)

	// Output:
	// true
}
