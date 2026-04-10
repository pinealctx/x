// Package handlerx provides a generic, framework-agnostic middleware chain for
// RPC handlers. It depends only on the errorx package within this module.
//
// Handler[Req, Resp] is the core function type. Interceptor[Req, Resp] wraps a
// Handler to inject logic before and after the call. Chain composes multiple
// interceptors around a handler, executing them outermost-first.
//
// Built-in interceptors: WithTimeout and WithRecovery.
package handlerx
