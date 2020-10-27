package main

import (
  //"errors"
  "fmt"
  "github.com/gin-gonic/gin"
  "os"
  "strconv"
  //"sync"
  //"io"
  "io/ioutil"
  "log"
  //"regexp"
  //"strings"
  //"time"

  bolt "go.etcd.io/bbolt"
  "net/http"
)

var http_user, http_password string
var db *bolt.DB

func get(c *gin.Context) {
  path := c.Param("path")
  var content []byte
  var ct string

  db.View(func(tx *bolt.Tx) error {
    b := tx.Bucket([]byte("wclip"))
    content = b.Get([]byte("content:" + path))
    ct = string(b.Get([]byte("content-type:" + path)))
    return nil
  })

  if content == nil {
    c.String(404, "Not found")
    return
  }

  c.Header("content-type", ct)
  c.String(200, string(content[:]))
}

func set(c *gin.Context) {
  path := c.Param("path")
  content, err := ioutil.ReadAll(c.Request.Body)
  if err != nil {
    c.String(500, "Error reading request: "+err.Error())
    return
  }
  ct := c.GetHeader("content-type")

  err = db.Update(func(tx *bolt.Tx) error {
    b := tx.Bucket([]byte("wclip"))
    err := b.Put([]byte("content:"+path), content)
    if err != nil {
      return err
    }
    err = b.Put([]byte("content-type:"+path), []byte(ct))
    if err != nil {
      return err
    }
    return nil
  })

  if err != nil {
    c.String(500, "Error saving: "+err.Error())
    return
  }
  c.String(http.StatusCreated, "Created")
}

func set_cors_headers(c *gin.Context) {
  c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
  c.Writer.Header().Set("Access-Control-Allow-Methods", "GET")
}

func main() {
  var err error

  http_user = os.Getenv("HTTP_USER")
  http_password = os.Getenv("HTTP_PASSWORD")
  if http_user == "" && http_password != "" {
    log.Fatal("HTTP_PASSWORD was specified but HTTP_USER was not, they need to be given together")
  }
  if http_user != "" && http_password == "" {
    log.Fatal("HTTP_USER was specified but HTTP_PASSWORD was not, they need to be given together")
  }

  db_path := os.Getenv("DB_PATH")
  if db_path == "" {
    db_path = "wclip.db"
  }
  db, err = bolt.Open(db_path, 0600, nil)
  if err != nil {
    log.Fatal("Error opening database")
  }
  defer db.Close()

  db.Update(func(tx *bolt.Tx) error {
    b, err := tx.CreateBucketIfNotExists([]byte("wclip"))
    if err != nil {
      log.Fatal("Cannot create wclip bucket")
    }
    b = b
    return nil
  })

  // Disable Console Color
  // gin.DisableConsoleColor()

  debug := os.Getenv("DEBUG")
  if debug == "" {
    gin.SetMode(gin.ReleaseMode)
  }

  // Creates a gin router with default middleware:
  // logger and recovery (crash-free) middleware
  router := gin.Default()

  //router.LoadHTMLGlob("views/*.html")

  //router.Use(gin.Recovery())

  router.GET("/*path", get)
  router.POST("/*path", set)
  router.PUT("/*path", set)
  //router.GET("/robots.txt", robots_txt)

  // By default it serves on :8080 unless a
  // PORT environment variable was defined.
  port := os.Getenv("PORT")
  var iport int
  if port == "" {
    iport = 8093
  } else {
    iport, err = strconv.Atoi(port)
    if err != nil {
      log.Fatal(err)
    }
  }
  router.Run(fmt.Sprintf(":%d", iport))
  // router.Run(":3000") for a hard coded port
}
