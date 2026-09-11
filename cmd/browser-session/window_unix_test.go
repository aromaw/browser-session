//go:build linux || darwin

package main

func fakeWindow() <-chan struct{} { return make(chan struct{}) }
