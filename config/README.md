# config

This package handles two related jobs:

- load startup config into a struct
- read config values later through a provider interface

Use it when a service needs one startup source but still wants a common API for file, env, or Rigel-backed config.

## Concepts

### Loader

A `Loader` reads startup config into a target struct.

Use:

- `config.NewFile(path)`
- `config.NewEnv(prefix)`
- `config.NewRigel(client)`

Then call:

```go
err := config.LoadWith(loader, &cfg)
```

### Provider

A `Provider` reads values at the point of use.

Methods:

- `Get(key)`
- `GetInt(key)`
- `GetBool(key)`
- `Watch(ctx, key, events)`

`File` and `Rigel` implement both loader and provider behavior.

`Env` supports `Get`, `GetInt`, and `GetBool`, but `Watch` returns an error.

### Backward compatibility

The older `Config` interface and `Load(...)` function still exist.

New code should prefer:

- `Loader`
- `Provider`
- `LoadWith(...)`

## Load startup config from a file

Given this file:

```json
{
  "database": {
    "host": "localhost",
    "port": 5432
  },
  "server": {
    "port": 8080
  }
}
```

Load it into a struct:

```go
package main

import "github.com/remiges-tech/alya/config"

type AppConfig struct {
    Database struct {
        Host string `json:"host"`
        Port int    `json:"port"`
    } `json:"database"`
    Server struct {
        Port int `json:"port"`
    } `json:"server"`
}

func loadConfig() (AppConfig, error) {
    var cfg AppConfig

    loader, err := config.NewFile("config.json")
    if err != nil {
        return cfg, err
    }
    if err := config.LoadWith(loader, &cfg); err != nil {
        return cfg, err
    }

    return cfg, nil
}
```

## Load startup config from environment variables

`Env` maps nested struct fields to uppercase environment variable names joined with underscores.

Example struct:

```go
type AppConfig struct {
    Database struct {
        Host string `json:"host"`
        Port int    `json:"port"`
    } `json:"database"`
    Server struct {
        Port int `json:"port"`
    } `json:"server"`
}
```

With prefix `ALYA_APP`, the package reads:

- `ALYA_APP_DATABASE_HOST`
- `ALYA_APP_DATABASE_PORT`
- `ALYA_APP_SERVER_PORT`

Load it like this:

```go
loader := config.NewEnv("ALYA_APP")

var cfg AppConfig
if err := config.LoadWith(loader, &cfg); err != nil {
    return err
}
```

You can also override the segment name with an `env` tag:

```go
type AppConfig struct {
    Server struct {
        Port int `json:"port" env:"http_port"`
    } `json:"server"`
}
```

That maps to:

- `ALYA_APP_SERVER_HTTP_PORT`

## Read values through Provider

Use a provider when code needs individual values instead of a full struct.

### File provider

```go
provider, err := config.NewFile("config.json")
if err != nil {
    return err
}

host, err := provider.Get("database.host")
if err != nil {
    return err
}

port, err := provider.GetInt("server.port")
if err != nil {
    return err
}

secure, err := provider.GetBool("server.secure")
if err != nil {
    return err
}
```

Notes:

- file keys can use dot-separated paths such as `database.host`
- direct key lookup still happens before dot-splitting
- `Get(...)` returns a string form for non-string values and also returns an error of type `*ValueNotStringError`

### Environment provider

```go
provider := config.NewEnv("ALYA_APP")

host, err := provider.Get("database.host")
port, err := provider.GetInt("server.port")
```

This reads:

- `ALYA_APP_DATABASE_HOST`
- `ALYA_APP_SERVER_PORT`

## Watch for updates

### Watch a JSON file key

`File.Watch(...)` monitors the parent directory of the config file and reloads the file when it sees write, create, or rename events.

It sends an event only when the resolved string value for the requested key changes.

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

provider, err := config.NewFile("config.json")
if err != nil {
    return err
}

events := make(chan config.Event, 1)
if err := provider.Watch(ctx, "server.port", events); err != nil {
    return err
}

go func() {
    for event := range events {
        log.Printf("config changed: %s=%s", event.Key, event.Value)
    }
}()
```

### Watch with Env

`Env.Watch(...)` is not supported.

It returns an error.

## Use Rigel through config

Wrap an existing Rigel client when you want to use the same loader and provider interfaces:

```go
rigelClient, err := config.NewRigelClient("localhost:2379")
if err != nil {
    return err
}

source := config.NewRigel(rigelClient)

var cfg AppConfig
if err := config.LoadWith(source, &cfg); err != nil {
    return err
}
```

You can also read values with `Get`, `GetInt`, `GetBool`, and `Watch` on the wrapped Rigel source.

## Common patterns

### Select config source at startup

```go
func newConfigLoader(source string) (config.Loader, error) {
    switch source {
    case "file":
        return config.NewFile("config.json")
    case "env":
        return config.NewEnv("ALYA_APP"), nil
    default:
        return nil, fmt.Errorf("unsupported source: %s", source)
    }
}
```

### Keep startup loading separate from runtime reads

A common split is:

- `LoadWith(...)` in `main.go` for startup config
- `Provider` in long-lived components that need direct reads or watch behavior

## Related examples

See:

- `examples/rest-usersvc-sqlc-example/main.go`
- `examples/wsc-usersvc-sqlc-example/cmd/service/main.go`

Both examples show file and environment startup loading.
