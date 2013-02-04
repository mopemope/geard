package dbus

import (
	"encoding/xml"
	"strings"
)

// Node is the root element of an introspection.
type Node struct {
	XMLName    xml.Name    `xml:"node"`
	Name       string      `xml:"name,attr,omitempty"`
	Interfaces []Interface `xml:"interface"`
	Children   []Node      `xml:"node,omitempty"`
}

// Interface describes a DBus interface that is available on the message bus.
type Interface struct {
	Name        string       `xml:"name,attr"`
	Methods     []Method     `xml:"method"`
	Signals     []SignalInfo `xml:"signal"`
	Properties  []Property   `xml:"property"`
	Annotations []Annotation `xml:"annotation"`
}

// Method describes a Method on an Interface as retured by an introspection.
type Method struct {
	Name        string       `xml:"name,attr"`
	Args        []Arg        `xml:"arg"`
	Annotations []Annotation `xml:"annotation"`
}

// SignalInfo describes a Signal emitted on an Interface.
type SignalInfo struct {
	Name        string       `xml:"name,attr"`
	Args        []Arg        `xml:"arg"`
	Annotations []Annotation `xml:"annotation"`
}

// Property describes a property of an Interface.
type Property struct {
	Name        string       `xml:"name,attr"`
	Type        string       `xml:"type,attr"`
	Access      string       `xml:"access,attr"`
	Annotations []Annotation `xml:"annotation"`
}

// Arg represents an argument of a method or a signal.
type Arg struct {
	Name      string `xml:"name,attr"`
	Type      string `xml:"type,attr"`
	Direction string `xml:"direction,attr"`
}

// Annotation is a annotation in the introspection format.
type Annotation struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

// Introspect calls org.freedesktop.Introspectable.Introspect on the given
// object and returns the introspection data.
func (o *Object) Introspect() (*Node, error) {
	var xmldata string
	var node Node

	err := o.Call("org.freedesktop.DBus.Introspectable.Introspect", 0).Store(&xmldata)
	if err != nil {
		return nil, err
	}
	err = xml.NewDecoder(strings.NewReader(xmldata)).Decode(&node)
	if err != nil {
		return nil, err
	}
	if node.Name == "" {
		node.Name = string(o.path)
	}
	return &node, nil
}