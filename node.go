package greact

// A NodeType is the type of a Node.
type NodeType uint32

// common HTML node types
const (
	ErrorNode NodeType = iota
	TextNode
	DocumentNode
	ElementNode
	CommentNode
	DoctypeNode
)

func (n NodeType) String() string {
	switch n {
	case ErrorNode:
		return "ErrorNode"
	case TextNode:
		return "TextNode"
	case DocumentNode:
		return "DocumentNode"
	case ElementNode:
		return "ElementNode"
	case CommentNode:
		return "CommentNode"
	case DoctypeNode:
		return "DoctypeNode"
	default:
		return "ErrorNode"
	}
}

// Attribute represent html attribute. Val can be any valid go expression.
type Attribute struct {
	Namespace, Key string
	Val            interface{}
}

// Node represents a virtual dom node.
type Node struct {
	Type      NodeType
	Data      string
	Namespace string
	Attr      []Attribute
	Children  []*Node
}

// NewNode is a wrapper for creating new node
func NewNode(typ NodeType, ns, name string, attrs []Attribute, children ...*Node) *Node {
	return &Node{
		Type:      typ,
		Namespace: ns,
		Data:      name,
		Attr:      attrs,
		Children:  newChildren(children...),
	}
}

// newChildren processes n nodes.
//
// Adjacent text nodes are merged.
func newChildren(n ...*Node) []*Node {
	if len(n) > 0 {
		var o []*Node
		var lastText *Node
		for _, v := range n {
			switch v.Type {
			case TextNode:
				if lastText != nil {
					lastText.Data += v.Data
				} else {
					lastText = v
					o = append(o, lastText)
				}
			default:
				lastText = nil
				o = append(o, v)
			}
		}
		return o
	}
	return nil
}

// Attr returns Attribute from the arguments. This doesn't do much appart from
// wrapping the arguments.
func Attr(ns, key string, val interface{}) Attribute {
	return Attribute{
		Namespace: ns,
		Key:       key,
		Val:       val,
	}
}

// Attrs is a wrapper/shortcut for optionally providing Attributes. Due tto the
// nature of composition for components, this saves space and improves
// readability.
func Attrs(attr ...Attribute) []Attribute {
	return attr
}

// Key returns the value of the key attribute of the node as a string. Key
// attributes can be set to allow easily identifying lists nodes for faster re
// re rendering.
func (v *Node) Key() string {
	for _, v := range v.Attr {
		if v.Key == "key" {
			return v.Val.(string)
		}
	}
	return ""
}
