// Package ava outputs ava service descriptions in Go code.
// It runs as a plugin for the Go protocol buffer compiler plugin.
// It is linked in to protoc-gen-go.

package ava

import (
	"fmt"
	"strings"

	pb "github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	"github.com/gogo/protobuf/protoc-gen-gogo/generator"
)

// generatedCodeVersion indicates a version of the generated code.
// It is incremented whenever an incompatibility between the generated code and
// the ava package is introduced; the generated code references
// a constant, ava.SupportPackageIsVersionN (where N is generatedCodeVersion).
const generatedCodeVersion = 1

// Paths for packages used by code generated in this file,
// relative to the import_prefix of the generator.Generator.
const (
	avaServicePkgPath = "github.com/aiakit/ava"
)

func init() {
	generator.RegisterPlugin(new(ava))
}

// ava is an implementation of the Go protocol buffer compiler's
// plugin architecture.  It generates bindings for ava support.
type ava struct {
	gen *generator.Generator
}

// Name returns the name of this plugin, "github.com/aiakit/ava".
func (r *ava) Name() string {
	return "ava"
}

// The names for packages imported in the generated code.
// They may vary from the final path component of the import path
// if the name is used by other packages.
var (
	avaServicePkg string
)

// Init initializes the plugin.
func (r *ava) Init(gen *generator.Generator) {
	r.gen = gen
}

// Given a type name defined in a .proto, return its object.
// Also record that we're using it, to guarantee the associated import.
func (r *ava) objectNamed(name string) generator.Object {
	r.gen.RecordTypeUse(name)
	return r.gen.ObjectNamed(name)
}

// Given a type name defined in a .proto, return its name as we will print it.
func (r *ava) typeName(str string) string {
	return r.gen.TypeName(r.objectNamed(str))
}

// P forwards to g.gen.P.
func (r *ava) P(args ...interface{}) { r.gen.P(args...) }

// Generate generates code for the services in the given file.
func (r *ava) Generate(file *generator.FileDescriptor) {
	if len(file.FileDescriptorProto.Service) == 0 {
		return
	}

	avaServicePkg = string(r.gen.AddImport(avaServicePkgPath))

	r.P("// Reference imports to suppress errors if they are not otherwise used.")
	r.P("var _ ", avaServicePkg, ".Service")
	r.P()

	// Assert version compatibility.
	r.P("// This is a compile-time assertion to ensure that this generated file")
	r.P("// is compatible with the ava package it is being compiled against.")
	r.P("const _ = ", avaServicePkg, ".SupportPackageIsVersion", generatedCodeVersion)
	r.P()

	for i, service := range file.FileDescriptorProto.Service {
		r.generateService(file, service, i)
	}
}

// GenerateImports generates the import declaration for this file.
func (r *ava) GenerateImports(file *generator.FileDescriptor) {}

func unexport(s string) string { return strings.ToLower(s[:1]) + s[1:] }

// deprecationComment is the standard comment added to deprecated
// messages, fields, enums, and enum values.
var deprecationComment = "// Deprecated: Do not use."

// generateService generates all the code for the named service.
func (r *ava) generateService(file *generator.FileDescriptor, service *pb.ServiceDescriptorProto, index int) {
	path := fmt.Sprintf("6,%d", index) // 6 means service.

	origServerName := service.GetName()
	fullServerName := origServerName
	if pkg := file.GetPackage(); pkg != "" {
		fullServerName = pkg + "." + fullServerName
	}
	serverName := generator.CamelCase(origServerName)
	deprecated := service.GetOptions().GetDeprecated()

	r.P()
	// service interface.
	if deprecated {
		r.P("//")
		r.P(deprecationComment)
	}
	r.P("type ", serverName, "Client interface {")
	for i, method := range service.Method {
		r.gen.PrintComments(fmt.Sprintf("%s,2,%d", path, i)) // 2 means method in a service.
		clientSignature := r.generateClientSignature(serverName, method)
		if clientSignature == "" {
			continue
		}
		r.P(clientSignature)
	}
	r.P("}")
	r.P()

	// service structure.
	r.P("type ", unexport(serverName), "Client struct {")
	r.P("c *", "ava.Client")
	r.P("}")
	r.P()

	// NewClient factory.
	if deprecated {
		r.P(deprecationComment)
	}
	r.P("func New", serverName, "Client () ", serverName, "Client {")
	r.P("return &", unexport(serverName), "Client{c:ava.AvaClient()}")
	r.P("}")
	r.P()

	// service method implementations.
	for _, method := range service.Method {
		r.generateClientMethod(serverName, method)
	}

	// Server interface.
	serverType := serverName + "Server"
	r.P("// ", serverType, " is the server API for ", serverName, " ava.")
	if deprecated {
		r.P("//")
		r.P(deprecationComment)
	}
	r.P("type ", serverType, " interface {")
	for i, method := range service.Method {
		r.gen.PrintComments(fmt.Sprintf("%s,2,%d", path, i)) // 2 means method in a service.
		r.P(r.generateServerSignature(method))
	}
	r.P("}")
	r.P()

	// Server registration.
	if deprecated {
		r.P(deprecationComment)
	}
	r.P("func Register", serverName, "Server( h ", serverType, ") {")
	r.P("var r = &", unexport(serverName), "Handler{h:h}")

	for _, v := range service.Method {
		if !v.GetClientStreaming() && !v.GetServerStreaming() {
			r.P(`ava.AvaServer().RegisterHandler("/"+ava.AvaServer().SceneName()+"/`, strings.ToLower(serverName), "/", strings.ToLower(*v.Name), `",r.`, *v.Name, ")")
		}
		if !v.GetClientStreaming() && v.GetServerStreaming() {
			r.P(`ava.AvaServer().RegisterStreamHandler("/"+ava.AvaServer().SceneName()+"/`, strings.ToLower(serverName), "/", strings.ToLower(*v.Name), `",r.`, *v.Name, ")")
		}

		if v.GetClientStreaming() && v.GetServerStreaming() {
			r.P(`ava.AvaServer().RegisterChannelHandler("/"+ava.AvaServer().SceneName()+"/`, strings.ToLower(serverName), "/", strings.ToLower(*v.Name), `",r.`, *v.Name, ")")
		}
	}
	r.P("}")
	r.P()

	r.P("type ", unexport(serverName), "Handler struct{")
	r.P("h ", serverName, "Server")
	r.P("}")
	r.P()

	for _, method := range service.Method {
		r.generateServerMethod(serverName, method)
	}
	r.P()
}

// generateClientSignature returns the client-side signature for a method.
func (r *ava) generateClientSignature(serverName string, method *pb.MethodDescriptorProto) string {
	var (
		origMethodName = method.GetName()
		methodName     = generator.CamelCase(origMethodName)
	)

	if !method.GetClientStreaming() && !method.GetServerStreaming() {
		var (
			reqArg   = ", req *" + r.typeName(method.GetInputType())
			respName = "*" + r.typeName(method.GetOutputType())
		)

		//if r.GetavaApiPrefix(methodName) {
		//    return ""
		//}

		return fmt.Sprintf(
			"%s(c *%s.Context%s, opts ...ava.InvokeOptions) (%s, error)",
			methodName,
			avaServicePkg,
			reqArg,
			respName,
		)
	}

	if !method.GetClientStreaming() && method.GetServerStreaming() {
		var (
			reqArg   = ", req *" + r.typeName(method.GetInputType())
			respName = "chan *" + r.typeName(method.GetOutputType())
		)
		return fmt.Sprintf(
			"%s(c *%s.Context%s, opts ...ava.InvokeOptions) %s",
			methodName,
			avaServicePkg,
			reqArg,
			respName,
		)
	}

	if method.GetClientStreaming() && method.GetServerStreaming() {
		var (
			reqArg   = ", req chan *" + r.typeName(method.GetInputType())
			respName = "chan *" + r.typeName(method.GetOutputType())
		)
		return fmt.Sprintf(
			"%s(c *%s.Context%s, opts ...ava.InvokeOptions) %s",
			methodName,
			avaServicePkg,
			reqArg,
			respName,
		)
	}

	return ""
}

func (r *ava) generateClientMethod(serverName string, method *pb.MethodDescriptorProto) {
	var (
		methodName = generator.CamelCase(method.GetName())
		outType    = r.typeName(method.GetOutputType())
	)

	if method.GetOptions().GetDeprecated() {
		r.P(deprecationComment)
	}

	if !method.GetServerStreaming() && !method.GetClientStreaming() {

		//if r.GetavaApiPrefix(methodName) {
		//    return
		//}

		r.P("func (cc *", unexport(serverName), "Client) ", r.generateClientSignature(serverName, method), "{")
		r.P("rsp := &", outType, "{}")
		r.P(`err := cc.c.InvokeRR(c, "/`, strings.ToLower(serverName), "/", strings.ToLower(methodName), `", req, rsp, opts...)`)
		r.P("return rsp, err")
		r.P("}")
		r.P()
		return
	}

	if !method.GetClientStreaming() && method.GetServerStreaming() {
		r.P("func (cc *", unexport(serverName), "Client) ", r.generateClientSignature(serverName, method), "{")
		r.P(`data :=cc.c.InvokeRS(c, "/`, strings.ToLower(serverName), "/", strings.ToLower(methodName), `", req, opts...)`)
		r.P("if data==nil{")
		r.P("return nil")
		r.P("}")
		r.P()
		r.P("var rsp = make(chan *", outType, ",cap(data))")
		r.P("go func() {")
		r.P("for b := range data {")
		r.P("v := &", outType, "{}")
		r.P("err :=  c.Codec().Decode(b, v)")
		r.P("if err != nil {")
		r.P(" c.Errorf(\"client decode pakcet err=%v |method=%s |data=%s\", err, c.Metadata.Method(), req.String())")
		r.P("continue")
		r.P("}")
		r.P("rsp <- v")
		r.P("}")
		r.P("close(rsp)")
		r.P("}()")
		r.P("return rsp")
		r.P("}")
		r.P()
	}

	if method.GetClientStreaming() && method.GetServerStreaming() {
		r.P("func (cc *", unexport(serverName), "Client) ", r.generateClientSignature(serverName, method), "{")
		r.P("var in = make(chan []byte,cap(req))")
		r.P(`data :=cc.c.InvokeRC(c, "/`, strings.ToLower(serverName), "/", strings.ToLower(methodName), `", in, opts...)`)
		r.P("if data==nil{")
		r.P("return nil")
		r.P("}")
		r.P()
		r.P("go func() {")
		r.P("for b := range req {")
		r.P("v, err := c.Codec().Encode(b)")
		r.P("if err != nil {")
		r.P(" c.Errorf(\"client encode pakcet err=%v |method=%s |data=%s\", err, c.Metadata.Method(), b.String())")
		r.P("continue")
		r.P("}")
		r.P("in <- v")
		r.P("}")
		r.P("close(in)")
		r.P("}()")
		r.P()
		r.P("var rsp = make(chan *", outType, ",cap(data))")
		r.P("go func() {")
		r.P("for b := range data {")
		r.P("v := &", outType, "{}")
		r.P("err := c.Codec().Decode(b, v)")
		r.P("if err != nil {")
		r.P(" c.Errorf(\"client decode pakcet err=%v |method=%s |data=%s\", err, c.Metadata.Method(), string(b))")
		r.P("continue")
		r.P("}")
		r.P("rsp <- v")
		r.P("}")
		r.P("close(rsp)")
		r.P("}()")
		r.P("return rsp")
		r.P("}")
		r.P()
	}
}

// generateServerSignature returns the server-side signature for a method.
func (r *ava) generateServerSignature(method *pb.MethodDescriptorProto) string {
	origMethodName := method.GetName()
	methodName := generator.CamelCase(origMethodName)

	var reqArgs []string
	if !method.GetServerStreaming() && !method.GetClientStreaming() {

		reqArgs = append(reqArgs, "c *"+avaServicePkg+".Context")
		reqArgs = append(reqArgs, "req *"+r.typeName(method.GetInputType()))
		reqArgs = append(reqArgs, "rsp *"+r.typeName(method.GetOutputType()))

		return methodName + "(" + strings.Join(
			reqArgs,
			", ",
		) + ")"
	}

	if !method.GetClientStreaming() && method.GetServerStreaming() {
		reqArgs = append(reqArgs, "c *"+avaServicePkg+".Context")
		reqArgs = append(reqArgs, "req *"+r.typeName(method.GetInputType()))
		return methodName + "(" + strings.Join(
			reqArgs,
			", ",
		) + ",exit chan struct{}) " + "chan *" + r.typeName(method.GetOutputType())
	}

	if method.GetClientStreaming() && method.GetServerStreaming() {
		reqArgs = append(reqArgs, "c *"+avaServicePkg+".Context")
		reqArgs = append(reqArgs, "req chan *"+r.typeName(method.GetInputType()))
		return methodName + "(" + strings.Join(
			reqArgs,
			", ",
		) + ",exit chan struct{}) " + "chan *" + r.typeName(method.GetOutputType())
	}

	return ""
}

func (r *ava) generateServerMethod(serverName string, method *pb.MethodDescriptorProto) {
	var (
		methodName = generator.CamelCase(method.GetName())
		inType     = r.typeName(method.GetInputType())
		outType    = r.typeName(method.GetOutputType())
	)

	if !method.GetServerStreaming() && !method.GetClientStreaming() {

		r.P(
			"func (r *",
			unexport(serverName),
			"Handler)",
			methodName,
			"(c *",
			avaServicePkg,
			".Context, req *ava.Packet,interrupt ava.Interceptor) (rsp proto.Message, err error) {",
		)
		r.P("var in ", inType)
		r.P("err = c.Codec().Decode(req.Bytes(), &in)")
		r.P("if err != nil {")
		r.P(" c.Errorf(\"server decode packet err=%v |method=%s |data=%s\", err, c.Metadata.Method(), req.String())")
		r.P("return nil,err")
		r.P("}")
		r.P("var out = ", outType, "{}")
		r.P("if interrupt == nil {")
		r.P("r.h.", methodName, "(c, &in,&out)")
		r.P("return &out, err")
		r.P("}")
		r.P("f := func(c *ava.Context, req proto.Message) proto.Message {")
		r.P("r.h.", methodName, "(c, req.(*", inType, "),&out)")
		r.P("return &out")
		r.P("}")
		r.P("return interrupt(c, &in, f)")
		r.P("}")
		r.P()
		return
	}

	if !method.GetClientStreaming() && method.GetServerStreaming() {
		r.P(
			"func (r *",
			unexport(serverName),
			"Handler)",
			methodName,
			"(c *",
			avaServicePkg,
			".Context, req *ava.Packet,exit chan struct{}) chan proto.Message {",
		)
		r.P("var in ", inType)
		r.P("err := c.Codec().Decode(req.Bytes(), &in)")
		r.P("if err != nil {")
		r.P(" c.Errorf(\"server decode packet err=%v |method=%s |data=%s\", err, c.Metadata.Method(), req.String())")
		r.P("return nil")
		r.P("}")
		r.P()
		r.P("out := r.h.", methodName, "(c, &in,exit)")
		r.P("if out==nil{")
		r.P("return nil")
		r.P("}")
		r.P()
		r.P("var rsp = make(chan proto.Message,cap(out))")
		r.P()
		r.P("go func() {")
		r.P("for d := range out {")
		r.P("rsp <- d")
		r.P("}")
		r.P("close(rsp)")
		r.P("}()")
		r.P("return rsp")
		r.P("}")
		r.P()
		return
	}

	if method.GetClientStreaming() && method.GetServerStreaming() {
		r.P(
			"func (r *",
			unexport(serverName),
			"Handler)",
			methodName,
			"(c *",
			avaServicePkg,
			".Context, req chan *ava.Packet,exit chan struct{}) chan proto.Message {",
		)
		r.P("var in = make(chan *", inType, ",cap(req))")
		r.P("out := r.h.", methodName, "(c, in,exit)")
		r.P("if out==nil{")
		r.P("return nil")
		r.P("}")
		r.P()
		r.P("go func() {")
		r.P("for b := range req {")
		r.P("var v = &", inType, "{}")
		r.P("err := c.Codec().Decode(b.Bytes(), v)")
		r.P("if err != nil {")
		r.P("c.Errorf(\"server decode packet err=%v |method=%s |data=%s\", err, c.Metadata.Method(), b.String())")
		r.P("continue")
		r.P("}")
		r.P("in <- v")
		r.P("ava.Recycle(b)")
		r.P("}")
		r.P("close(in)")
		r.P("}()")
		r.P("var rsp = make(chan proto.Message,cap(out))")
		r.P()
		r.P("go func() {")
		r.P("for d := range out {")
		r.P("rsp <- d")
		r.P("}")
		r.P("close(rsp)")
		r.P("}()")
		r.P("return rsp")
		r.P("}")
		r.P()
		return
	}
}
