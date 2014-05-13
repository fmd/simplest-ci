package main

import (
    "github.com/go-martini/martini"
    "github.com/fsouza/go-dockerclient"
    "github.com/martini-contrib/render"
    "github.com/garyburd/redigo/redis"
    "io/ioutil"
    "net/http"
    "errors"
    "time"
    "fmt"
)

var imageName string = "fmdud/ot-booking"

type EvCi struct {
    Client *docker.Client
    Conn redis.Conn
}

func main() {
    e := &EvCi{}

    c, err := redis.DialTimeout("tcp", ":6379", 0, 1*time.Second, 1*time.Second) 
    if err != nil {
        panic(err)
    }

    e.Conn = c

    endpoint := "unix:///var/run/docker.sock"
    client, err := docker.NewClient(endpoint)
    if err != nil {
        panic(err)
    }

    e.Client = client

    m := martini.Classic()
    m.Use(render.Renderer())

    m.Get("/", e.getInstances)
    m.Get("/new", e.newInstance)
    m.Get("/:id", e.getInstance)
    m.Get("/:id/build",e.getBuild)
    m.Get("/:id/spec",e.getSpec)
    m.Get("/:id/api",e.getApi)
    m.Get("/:id/db",e.getDb)

    m.Run()
}

func (e *EvCi) getInstance(r render.Render, params martini.Params) {    
    res, err := redis.String(e.Conn.Do("GET", params["id"]))
    if err != nil {
        r.JSON(500, err)
        return
    }

    r.JSON(200, string(res))
}

func (e *EvCi) getBuild(r render.Render, params martini.Params) string {
    res, err := redis.String(e.Conn.Do("GET", fmt.Sprintf("%s.build", params["id"])))
    if err != nil {
        r.JSON(500, err)
        return ""
    }

    return res
}

func (e *EvCi) getSpec(r render.Render, params martini.Params) string {
    res, err := redis.String(e.Conn.Do("GET", fmt.Sprintf("%s.spec", params["id"])))
    if err != nil {
        r.JSON(500, err)
        return ""
    }

    return res
}

func (e *EvCi) getApi(r render.Render, params martini.Params) string {
    res, err := redis.String(e.Conn.Do("GET", fmt.Sprintf("%s.api", params["id"])))
    if err != nil {
        r.JSON(500, err)
        return ""
    }

    return res
}

func (e *EvCi) getDb(r render.Render, params martini.Params) string {
    res, err := redis.String(e.Conn.Do("GET", fmt.Sprintf("%s.db", params["id"])))
    if err != nil {
        r.JSON(500, err)
        return ""
    }

    return res
}

func (e *EvCi) getInstances(r render.Render) {
    res, err := e.Conn.Do("LRANGE", "docker-tests", 0, -1)
    if err != nil {
        r.JSON(500, err)
        return
    }

    s, _ := res.([]string)

    r.JSON(200, s)
}

func (e *EvCi) removeRedis(id string) error {
    return nil
}

func (e *EvCi) setStatus(id string, status string) {
    e.Conn.Do("SET", fmt.Sprintf("%s",id), status)
}

func (e *EvCi) removeCI(id string) error {
    return e.Client.RemoveContainer(docker.RemoveContainerOptions{ID: id, Force: true})
}

func (e *EvCi) harvestCI(id string) error {
    container, err := e.Client.InspectContainer(id)
    if err != nil {
        e.removeCI(id)
        return err
    }
    
    ip := container.NetworkSettings.IPAddress

    e.setStatus(id, "in-progress")

    respErr := errors.New("")
    elapsed := 0
    
    for respErr != nil && elapsed < 300 {        
        _, respErr = http.Get(fmt.Sprintf("http://%s:8080/",ip))
        time.Sleep(1*time.Second)
        elapsed += 1

    }

    var resp *http.Response
    
    //Get build output
    resp, err = http.Get(fmt.Sprintf("http://%s:8080/build_output",ip))
    if err != nil {
        e.removeCI(id)
        e.setStatus(id, "error")
        return err
    }
    
    build, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        e.removeCI(id)
        e.setStatus(id, "error")
        return err
    }

    resp.Body.Close()
    
    //Get spec output
    resp, err = http.Get(fmt.Sprintf("http://%s:8080/spec_output",ip))
    if err != nil {
        e.removeCI(id)
        e.setStatus(id, "error")
        return err
    }

    spec, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        e.removeCI(id)
        e.setStatus(id, "error")
        return err
    }
    

    resp.Body.Close()

    //Get API output
    resp, err = http.Get(fmt.Sprintf("http://%s:8080/api_output",ip))
    if err != nil {
        e.removeCI(id)
        e.setStatus(id, "error")
        return err
    }
    

    api, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        e.removeCI(id)
        e.setStatus(id, "error")
        return err
    }

    resp.Body.Close()
    
    
    //Get DB output
    resp, err = http.Get(fmt.Sprintf("http://%s:8080/db_output",ip))
    if err != nil {
        e.removeCI(id)
        e.setStatus(id, "error")
        return err
    }
    

    db, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        e.removeCI(id)
        e.setStatus(id, "error")
        return err
    }
    resp.Body.Close()
    

    //Set to complete.
    e.setStatus(id, "complete")
    
    e.Conn.Do("SET", fmt.Sprintf("%s.%s",id,"build"), string(build))
    e.Conn.Do("SET", fmt.Sprintf("%s.%s",id,"api"), string(api))
    e.Conn.Do("SET", fmt.Sprintf("%s.%s",id,"spec"), string(spec))
    e.Conn.Do("SET", fmt.Sprintf("%s.%s",id,"db"), string(db))

    return e.removeCI(id)
}

func (e *EvCi) runCI() (string, error) {
    config := docker.Config{Image: imageName, Cmd:[]string{"/main.sh"}}
    container, err := e.Client.CreateContainer(docker.CreateContainerOptions{Config: &config})
    if err != nil {
        return "", err
    }

    err = e.Client.StartContainer(container.ID, &docker.HostConfig{})
    if err != nil {
        e.removeCI(container.ID)
        return "", err
    }

    _, err = e.Conn.Do("LPUSH","docker-tests",container.ID)
    if err != nil {
        e.removeCI(container.ID)
        return "", err
    }

    return container.ID, nil
}

func (e *EvCi) newInstance(r render.Render) {
    id, err := e.runCI()
    if err != nil {
        r.JSON(500, err)
        return
    }

    go e.harvestCI(id)
    r.JSON(200, id)
}