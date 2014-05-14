package main

import (
    "github.com/docopt/docopt-go"
    "github.com/go-martini/martini"
    "github.com/fsouza/go-dockerclient"
    "github.com/garyburd/redigo/redis"
    "time"
    "fmt"
)

var version string = "0.0.1"

func usage() string {
    return `simplest-ci
        Usage:
            simplest-ci [--port=<port>] [--redis=<redis>]
            simplest-ci --help
            simplest-ci --version

        Options:
            -r | --redis The redis host string [default: localhost:6379].
            -p | --port  The port to listen on [default: 5000].
            --help       Show this screen.
            --version    Show version.`
}

type Simplest struct {
    Redis *redis.Conn
    Docker *docker.Client
}

func main() {
    args, _ := docopt.Parse(usage(), nil, true, fmt.Sprintf("simplest-ci %s", version), false)

    redis := args["--redis"].(string)
    port := args["--port"].(string)

    s := &Simplest{}
    s.Redis, err := redis.DialTimeout("tcp", redis, 0, 1*time.Second, 1*time.Second)
    if err != nil {
        panic(err)
    }

    endpoint := "unix:///var/run/docker.sock"
    s.Docker, err = docker.NewClient(endpoint)
    if err != nil {
        panic(err)
    }

    m := martini.Classic()
    m.Run()
}