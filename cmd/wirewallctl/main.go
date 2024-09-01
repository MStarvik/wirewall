package main

import "github.com/godbus/dbus/v5"

const dbusName = "no.mstarvik.wirewall"
const dbusPath = "/no/mstarvik/wirewall"

func main() {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	obj := conn.Object(dbusName, dbusPath)
	call := obj.Call(dbusName+".Configure", 0)
	if call.Err != nil {
		panic(call.Err)
	}

	// var reply string
	// call.Store(&reply)

	// println(reply)
}
