package main

import (
	"database/sql"
	"expvar"
	"runtime"
	"time"
)

func expvarPublish(db *sql.DB) {
	expvar.Publish("version", expvar.Func(func() any {
		return version
	}))

	expvar.Publish("goroutines", expvar.Func(func() any {
		return runtime.NumGoroutine()
	}))

	expvar.Publish("database", expvar.Func(func() any {
		return db.Stats()
	}))

	expvar.Publish("timestamp", expvar.Func(func() any {
		return time.Now().Unix()
	}))
}
