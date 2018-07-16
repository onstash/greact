// Package vected is a component based frontend framework for golang. This
// framework delivers high performance and sleek ui's, that works both on the
// serverside and the frontend.
//
// Go templates are used as the main templating system. The framework is
// inspired by react, especially preact which I used to learn more about how
// react works.
//
// Also, this borrows from vue js. The templates are just normal go templates so
// no need to learn a different syntax.
//
// The user intrface styles is ant design
// see https://github.com/ant-design/ant-design to learn more about ant design.
package vected

import (
	"github.com/gernest/vected/lib/props"
)

// Component is an interface which defines a unit of user interface.
type Component interface {

	// New must return an initialized component. This acts as a constructor, the
	// props passed to the component from the parent are passed as arguments,
	//
	// Initializing state should happen here.
	New(props.Props) (Component, error)

	// Template this is the vected template that is rendered by the component.
	Template() string

	// all components must embed the Core struct to satisfy this interface.xw
	core()
}

type Core struct{}

func (c *Core) core() {}
