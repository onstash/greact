// Package vected is a component based frontend framework for golang. This
// framework delivers high performance and responsive ui.
//
// This relies on the experimental wasm api to interact with dom. This started
// as a port of preact to go, but has since evolved. It still borrows a similar
// API from react/preact.
package vected

import (
	"container/list"
	"context"
	"strings"
	"sync"

	"github.com/gernest/vected/elements"

	"github.com/gernest/vected/prop"
	"github.com/gernest/vected/state"
)

// RenderMode is a flag determining how a component is rendered.
type RenderMode uint

//supported render mode
const (
	No RenderMode = iota
	Force
	Sync
	Async
)

// AttrKey is a key used to store node's attributes/props
const AttrKey = "__vected_attr__"

// This tracks the last id issued. We use sync pool to reuse component id's.
//
// TODO: come up with a better way that can scale.
var idx int
var idPool = &sync.Pool{
	New: func() interface{} {
		idx++
		return idx
	},
}

// Component is an interface which defines a unit of user interface.There are
// two ways to satisfy this interface.
//
// You can define a struct that embeds Core and implements Templater interface
// like this.
// 	type Foo struct {
// 		vected.Core
// 	}
//
// 	func (f Foo) Template() string {
// 		return `<div />`
// 	}
//
// Then run
//	vected render /path/to/foo package
// The command will automatically generate the Render method for you. For the
// example above it will generate something like this.
//
// 	var H = New
// 	func (f Foo) Render(ctx context.Context, props prop.Props, state state.State) *Node {
// 		return H(3, "", "div", nil)
// 	}
//
// The second way is to implement Render method. I recommend you stick with only
// implementing Templater interface which is less error prone and reduces
// verbosity.
type Component interface {
	Render(context.Context, prop.Props, state.State) *Node
	core() *Core
}

// Templater is an interface for describing components with xml like markup. The
// markup is similar to jsx but tailored towards go constructs.
type Templater interface {
	// Template returns jsx like string template. The template is compiled to
	// Render method of the struct that implements this interface..
	Template() string
}

// Constructor is an interface for creating new component instance.
type Constructor interface {
	New() Component
}

// Core is th base struct that every struct that wants to implement Component
// interface must embed.
//
// This is used to make Props available to the component.
type Core struct {
	id int

	// constructor is the name of the higher order component. This is can be
	// defined when registering components with Vected.Register. This library uses
	// golang.org/x/net/html for parsing component template, which defaults all
	// elements to lower case, so the constructor name must be lower case.
	constructor string

	context context.Context
	props   prop.Props
	state   state.State

	prevContext context.Context
	prevProps   prop.Props
	prevState   state.State

	// A list of functions that will be called after the component has been
	// rendered.
	renderCallbacks []func()

	// This is the instance of the child component.
	component       Component
	parentComponent Component

	// The base dom node on which the component was rendered. When this is set it
	// signals for an update, this will be nil if the component hasn't been
	// rendered yet.
	base     Element
	nextBase Element

	dirty   bool
	disable bool

	// Optional prop that must be unique among child components for efficient
	// rendering of lists.
	key prop.NullString

	// This is a callback used to receive instance of Component or the Dom element.
	// after they have been mounted.
	ref func(interface{})

	// priority this is a number indicating how important this component is in the
	// re rendering queue. The higher the number the more urgent re renders.
	priority int

	enqueue *queuedRender
}

func (c *Core) core() *Core { return c }

// SetState updates component state and schedule re rendering.
func (c *Core) SetState(newState state.State, callback ...func()) {
	prev := c.prevState
	c.prevState = newState
	c.state = state.Merge(prev, newState)
	if len(callback) > 0 {
		c.renderCallbacks = append(c.renderCallbacks, callback...)
	}
	c.enqueue.enqueueCore(c)
}

// Props returns current props.s
func (c *Core) Props() prop.Props {
	return c.props
}

// State returns current state.
func (c *Core) State() state.State {
	return c.state
}

// Context returns current context.
func (c *Core) Context() context.Context {
	return c.context
}

// InitState is an interface for exposing initial state.
// Component should implement this interface if they want to set initial state
// when the component is first created before being rendered.
type InitState interface {
	InitState() state.State
}

// InitProps is an interface for exposing default props. This will be merged
// with other props before being sent to render.
type InitProps interface {
	InitProps() prop.Props
}

// WillMount is an interface defining a callback which is invoked before the
// component is mounted on the dom.
type WillMount interface {
	ComponentWillMount()
}

// DidMount is an interface defining a callback that is invoked after the
// component has been mounted to the dom.
type DidMount interface {
	ComponentDidMount()
}

// WillUnmount is an interface defining a callback that is invoked prior to
// removal of the rendered component from the dom.
type WillUnmount interface {
	ComponentWillUnmount()
}

// WillReceiveProps is an interface defining a callback that will be called with
// the new props before they are accepted and passed to be rendered.
type WillReceiveProps interface {
	ComponentWillReceiveProps(context.Context, prop.Props)
}

// ShouldUpdate is an interface defining callback that is called before render
// determine if re render is necessary.
type ShouldUpdate interface {
	// If this returns false then re rendering for the component is skipped.
	ShouldComponentUpdate(context.Context, prop.Props, state.State) bool
}

// WillUpdate is an interface defining a callback that is called before rendering
type WillUpdate interface {
	// If returned props are not nil, then it will be merged with nextprops then
	// passed to render for rendering.
	ComponentWillUpdate(context.Context, prop.Props, state.State) prop.Props
}

// DidUpdate defines a callback that is invoked after rendering.
type DidUpdate interface {
	ComponentDidUpdate(prevProps prop.Props, prevState state.State)
}

// DerivedState is an interface which can be used to derive state from props.
type DerivedState interface {
	DeriveState(prop.Props, state.State) state.State
}

// WithContext is an interface used to update the context that is passed to
// component's children.
type WithContext interface {
	WithContext(context.Context) context.Context
}

type queuedRender struct {
	components *list.List
	mu         sync.RWMutex
	closed     bool
	v          *Vected
}

func newQueuedRender(v *Vected) *queuedRender {
	return &queuedRender{
		components: list.New(),
		v:          v,
	}
}

func (q *queuedRender) Push(v Component) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.components.PushBack(v)
}

// Pop returns the last added component and removes it from the queue.
func (q *queuedRender) Pop() Component {
	e := q.pop()
	if e != nil {
		return e.Value.(Component)
	}
	return nil
}

func (q *queuedRender) pop() *list.Element {
	e := q.last()
	q.mu.Lock()
	if e != nil {
		q.components.Remove(e)
	}
	q.mu.Unlock()
	return e
}

func (q *queuedRender) last() *list.Element {
	q.mu.RLock()
	e := q.components.Back()
	q.mu.RUnlock()
	return e
}

// Last returns the last added component to the queue.
func (q *queuedRender) Last() Component {
	e := q.last()
	if e != nil {
		return e.Value.(Component)
	}
	return nil
}

// Rerender re renders all enqueued dirty components async.
func (q *queuedRender) Rerender() {
	go q.rerender()
}

func (q *queuedRender) enqueue(cmp Component) {
	if !cmp.core().dirty {
		cmp.core().dirty = true
	}
	q.Push(cmp)
	q.Rerender()
}

func (q *queuedRender) enqueueCore(core *Core) {
	cmp := q.v.cache[core.id]
	if !cmp.core().dirty {
		cmp.core().dirty = true
	}
	q.Push(cmp)
	q.Rerender()
}

func (q *queuedRender) rerender() {
	for cmp := q.Pop(); cmp != nil; cmp = q.Pop() {
		if cmp.core().dirty {
			q.v.renderComponent(cmp, 0, false, false)
		}
	}
}

// Vected this is the ultimate struct that ports preact to work with go/was.
// This is not a direct port, the two languages are different. Although some
// portion of the methods are a direct translation, the working differs from
// preact.
type Vected struct {

	// queue this is q queue of components that are supposed to be rendered
	// asynchronously.
	queue *queuedRender

	// Component this is a mapping of component name to component instance. The
	// name is not case sensitive and must not be the same as the standard html
	// elements.
	//
	// This means you cant have a component with name div,p,h1 etc. Remember that
	// it  is case insensitive so Div is also not allowed.
	//
	// In case you are not so sure, use the github.com/gernest/elements package to
	// check if the name is a valid component.
	//
	// The registered components won't be used as is, instead new instance will be
	// created so please don't pass component's which have state in them
	// (initialized field values etc) here, because they will be ignored.
	components map[string]Component

	// mounts is a list of components ready to be mounted.
	mounts *list.List

	isSVGMode bool
	hydrating bool
	diffLevel int

	cache map[int]Component
	refs  map[int]int

	cb CallbackGenerator
}

// New returns an initialized Vected instance.
func New() *Vected {
	v := &Vected{
		cache:      make(map[int]Component),
		refs:       make(map[int]int),
		mounts:     list.New(),
		components: make(map[string]Component),
	}
	v.queue = newQueuedRender(v)
	return v
}

func (v *Vected) enqueueRender(cmp Component) {
	if cmp.core().dirty {
		v.queue.Push(cmp)
		v.queue.Rerender()
	}
}

func (v *Vected) flushMounts() {
	for c := v.mounts.Back(); c != nil; c = v.mounts.Back() {
		if cmp, ok := c.Value.(Component); ok {
			if m, ok := cmp.(DidMount); ok {
				m.ComponentDidMount()
			}
		}
		v.mounts.Remove(c)
	}
}

func (v *Vected) recollectNodeTree(node Element, unmountOnly bool) {
	cmp := v.findComponent(node)
	if cmp != nil {
		v.unmountComponent(cmp)
	} else {
		if !unmountOnly || !Valid(node.Get(AttrKey)) {
			RemoveNode(node)
		}
		v.removeChildren(node)
	}
}

// UndefinedFunc is a function  that returns a javascript undefined value.
type UndefinedFunc func() Value

// Undefined is a work around to allow the library to work with/without wasm
// support.
//
// TODO: find a better way to handle this.
var Undefined UndefinedFunc

func (v *Vected) diffAttributes(node Element, attrs, old []Attribute) {
	a := mapAtts(attrs)
	b := mapAtts(old)
	for k, val := range b {
		if _, ok := a[k]; !ok {
			SetAccessor(v.cb, node, k, val, Undefined(), v.isSVGMode)
		}
	}
	for k := range a {
		switch k {
		case "children", "innerHTML":
			continue
		default:
			SetAccessor(v.cb, node, k, b[k], a[k], v.isSVGMode)
		}
	}
}

func mapAtts(attrs []Attribute) map[string]Attribute {
	m := make(map[string]Attribute)
	for _, v := range attrs {
		m[v.Key] = v
	}
	return m
}

func (v *Vected) diff(ctx context.Context, elem Element, node *Node, parent Element, mountAll, componentRoot bool) Element {
	if v.diffLevel == 0 {
		v.diffLevel++
		// when first starting the diff, check if we're diffing an SVG or within an SVG
		v.isSVGMode = parent != nil && parent.Type() != TypeNull &&
			Valid(parent.Get("ownerSVGElement"))

		// hydration is indicated by the existing element to be diffed not having a
		// prop cache
		v.hydrating = Valid(elem) && Valid(elem.Get(AttrKey))
	}
	ret := v.idiff(ctx, elem, node, mountAll, componentRoot)

	// append the element if its a new parent
	if Valid(parent) &&
		!IsEqual(ret.Get("parentNode"), parent) {
		parent.Call("appendChild", ret)
	}
	v.diffLevel--
	if v.diffLevel == 0 {
		v.hydrating = false
		if !componentRoot {
			v.flushMounts()
		}
	}
	return ret
}

func (v *Vected) idiff(ctx context.Context, elem Element, node *Node, mountAll, componentRoot bool) Element {
	out := elem
	prevSVGMode := v.isSVGMode
	switch node.Type {
	case TextNode:
		if Valid(elem) && Valid(elem.Get("splitText")) &&
			Valid(elem.Get("parentNode")) {
			v := elem.Get("nodeValue").String()
			if v != node.Data {
				elem.Set("nodeValue", node.Data)
			}

		} else {
			out = Document.Call("createTextNode", node.Data)
			if Valid(elem) {
				if Valid(elem.Get("parentNode")) {
					elem.Get("parentNode").Call("replaceChild", out, elem)
				}
				v.recollectNodeTree(elem, true)
			}
		}
		out.Set(AttrKey, true)
		return out
	case ElementNode:
		if v.isHigherOrder(node) {
			return v.buildComponentFromVNode(ctx, elem, node, mountAll, false)
		}
		if !elements.Valid(node.Data) {
			if node.Data == "svg" {
				v.isSVGMode = true
			} else if node.Data == "foreignObject" {
				v.isSVGMode = false
			}
		}
		nodeName := node.Data
		if !Valid(elem) || !isNamedNode(elem, node) {
			out = CreateNode(nodeName)
			if Valid(elem) {
				if Valid(elem.Get("firstChild")) {
					out.Call("appendChild", elem.Get("firstChild"))
				}
				if e := elem.Get("parentNode"); Valid(e) {
					elem.Get("parentNode").Call("replaceChild", out, elem)
				}
				v.recollectNodeTree(elem, true)
			}
		}
		fc := out.Get("firstChild")
		props := out.Get(AttrKey)
		var old []Attribute
		if !Valid(props) {
			a := elem.Get("attributes")
			for _, v := range Keys(a) {
				old = append(old, Attribute{
					Key: v,
					Val: a.Get(v).String(),
				})
			}
		}
		if !v.hydrating && len(node.Children) == 1 &&
			node.Children[0].Type == TextNode && Valid(fc) &&
			Valid(fc.Get("splitText")) &&
			fc.Get("nextSibling").Type() == TypeNull {
			nv := node.Children[0].Data
			fv := fc.Get("nodeValue").String()
			if fv != nv {
				fc.Set("nodeValue", nv)
			}
		} else if len(node.Children) > 0 || Valid(fc) {
			v.innerDiffMode(ctx, out, node.Children, mountAll, v.hydrating)
		}
		v.diffAttributes(out, node.Attr, old)
		v.isSVGMode = prevSVGMode
		return out
	default:
		panic("Un supported node")
	}
}

func (v *Vected) buildComponentFromVNode(ctx context.Context, elem Element, node *Node, mountAll, componentRoot bool) Element {
	c := v.findComponent(elem)
	originalComponent := c
	oldElem := elem
	isDirectOwner := c != nil && c.core().constructor == node.Data
	isOwner := isDirectOwner
	props := getNodeProps(node)
	for {
		if c != nil && !isOwner {
			c = c.core().parentComponent
			if c != nil {
				isOwner = c.core().constructor == node.Data
				continue
			}
		}
		break
	}
	if c != nil && isOwner && (!mountAll || c.core().component != nil) {
		v.setProps(ctx, c, props, Async, mountAll)
		elem = c.core().base
	} else {
		if originalComponent != nil && !isDirectOwner {
			v.unmountComponent(originalComponent)
			elem = nil
			oldElem = nil
		}
		c = v.createComponentByName(ctx, node.Data, props)
		if elem != nil && !Valid(c.core().nextBase) {
			c.core().nextBase = elem
			oldElem = nil
		}
		v.setProps(ctx, c, props, Sync, mountAll)
		elem = c.core().base
		if oldElem != nil && !IsEqual(elem, oldElem) {
			//TODO dereference the component.
			oldElem.Set(componentKey, 0)
			v.recollectNodeTree(oldElem, false)
		}
	}
	return elem
}

func (v *Vected) innerDiffMode(ctx context.Context, elem Element, vchildrens []*Node, mountAll, isHydrating bool) {
	original := elem.Get("childNodes")
	length := original.Get("length").Int()
	keys := make(map[string]Element)
	var children []Element
	var min int
	if length > 0 {
		for i := 0; i < length; i++ {
			child := original.Index(i)
			cmp := v.findComponent(child)
			var key prop.NullString
			if cmp != nil {
				key = cmp.core().key
			}
			if !key.IsNull {
				keys[key.Value] = child
			} else {
				var x bool
				if cmp != nil || Valid(child.Get("splitText")) {
					v := child.Get("nodeValue").String()
					v = strings.TrimSpace(v)
					if isHydrating {
						x = v != ""
					} else {
						x = true
					}
				} else {
					x = isHydrating
				}
				if x {
					children = append(children, child)
				}
			}
		}
	}
	for i := 0; i < len(vchildrens); i++ {
		vchild := vchildrens[i]
		key := vchild.Key()
		var child Element
		if key != "" {
			if ch, ok := keys[key]; ok {
				delete(keys, key)
				child = ch
			}
		} else if min < len(children) {
			for j := min; j < len(children); j++ {
				c := children[j]
				if c != nil && Valid(c) && isSameNodeType(c, vchild, isHydrating) {
					child = c
					children[j] = nil
					if j == min {
						min++
					}
					break
				}
			}
		}
		child = v.idiff(ctx, child, vchild, mountAll, false)
		f := original.Index(i)
		if Valid(child) && !IsEqual(child, elem) && !IsEqual(child, f) {
			if f.Type() == TypeNull {
				elem.Call("appendChild", child)
			} else if IsEqual(child, f.Get("nextSibling")) {
				RemoveNode(f)
			} else {
				elem.Call("insertBefore", child, f)
			}
		}
	}

	// removing unused keyed  children
	for _, val := range keys {
		v.recollectNodeTree(val, false)
	}
	for i := min; i < len(children); i++ {
		ch := children[i]
		if ch != nil {
			v.recollectNodeTree(ch, false)
		}
	}
}

// isSameNodeType compares elem to vnode and returns true if thy are of the same
// type.
//
// There are only two types of nodes supported , TextNode and ElementNode.
func isSameNodeType(elem Element, vnode *Node, isHydrating bool) bool {
	switch vnode.Type {
	case TextNode:
		return Valid(elem.Get("splitText"))
	case ElementNode:
		return isNamedNode(elem, vnode)
	default:
		return false
	}
}

// isNamedNode compares elem to vnode to see if elem was created from the
// virtual node of the same type as vnode..
func isNamedNode(elem Element, vnode *Node) bool {
	v := elem.Get("normalizedNodeName")
	if Valid(v) {
		name := v.String()
		return name == vnode.Data
	}
	return false
}
