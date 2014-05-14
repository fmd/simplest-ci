package main

import (
    "github.com/docopt/docopt-go"
    "github.com/go-martini/martini"
    "github.com/fsouza/go-dockerclient"
    "github.com/garyburd/redigo/redis"
    "encoding/json"
    "net/http"
    "time"
    "log"
    "fmt"
    "os"
)

var version string = "0.0.1"

func usage() string {
    return `simplest-ci
        Usage:
            simplest-ci daemon [--redis=<redis>] [--port=<port>]
            simplest-ci run [--config=<config>]
            simplest-ci --help
            simplest-ci --version

        Options:
            -p | --port    The port to listen on.         [default: 5000]
            -r | --redis   The redis host string.         [default: localhost:6379]
            -c | --config  The config file to serve from. [default: simplest-config.json]
            --help         Show this screen.
            --version      Show version.`
}

type Config struct {
    Commit   string     `json:"commit"`
    Commands [][]string `json:"commands"`
}

type Simplest struct {
    Redis redis.Conn
    Docker *docker.Client
}

//runFromConfig takes a simplest config file and uses it to
//Update git, check out the correct commit, build the project and run the tests.
func runFromConfig(configFile string) error {
    file, err := os.Open(configFile)
    if err != nil {
        return err
    }

    conf := &Config{}

    parser := json.NewDecoder(file)
    err = parser.Decode(&conf)
    if err != nil {
        return err
    }

    fmt.Println(conf)

    return nil
}

//serveFromConfig starts a daemon uses redis and docker to start up instances of a project from testing.
func serveFromConfig(redisHost string, servePort string) {

    var err error

    //Create our main struct
    s := &Simplest{}

    //Connect to redis
    s.Redis, err = redis.DialTimeout("tcp", redisHost, 0, 1*time.Second, 1*time.Second)
    if err != nil {
        panic(err)
    }

    //Connect to docker
    endpoint := "unix:///var/run/docker.sock"
    s.Docker, err = docker.NewClient(endpoint)
    if err != nil {
        panic(err)
    }

    //Drink a martini
    m := martini.Classic()
    log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s",servePort), m))
}

func main() {

    //Parse args
    args, _ := docopt.Parse(usage(), nil, true, fmt.Sprintf("simplest-ci %s", version), false)

    //See what we're doing.
    if args["run"].(bool) {
        runFromConfig(args["--config"].(string))
    } else {
        redisHost := args["--redis"].(string)
        servePort := args["--port"].(string)
        serveFromConfig(redisHost, servePort)
    }
}

func (s *Simplest) newInstance(image string, path string, commit string) {

}