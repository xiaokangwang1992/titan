/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2024/01/23 14:58:11
 Desc     : define the context for monitor
*/

package context

import (
	"context"
)

type Context struct {
	context.Context
	parentContext *Context
	RestartWeb    chan bool
}

func NewContext(parentCtx *Context) *Context {
	c := &Context{RestartWeb: make(chan bool, 1), parentContext: parentCtx}
	if parentCtx != nil {
		c.Context = parentCtx.Context
	}
	if c.Context == nil {
		c.Context = context.Background()
	}

	return c
}
