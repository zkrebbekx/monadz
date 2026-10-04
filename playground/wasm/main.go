//go:build js && wasm

// Command wasm is the bridge between the monadz playground page and the
// scenarios package. It registers one function per scenario on the global
// "monadz" object. Each function takes a JSON string and returns a JSON
// string: {"ok":true,"data":...} or {"ok":false,"error":"..."}. The Task
// scenarios run real goroutines and timers, so their functions return a
// Promise of that string.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"syscall/js"
	"time"

	"github.com/zkrebbekx/monadz/playground/scenarios"
)

type envelope struct {
	Ok    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

func encode(e envelope) string {
	b, err := json.Marshal(e)
	if err != nil {
		b, _ = json.Marshal(envelope{Error: "encode: " + err.Error()})
	}
	return string(b)
}

// call decodes the input, runs f, and encodes the output. It recovers every
// panic, so that a bad input never stops the WASM instance.
func call[In, Out any](input string, f func(In) Out) (out string) {
	defer func() {
		if p := recover(); p != nil {
			out = encode(envelope{Error: fmt.Sprint("panic: ", p)})
		}
	}()
	var in In
	if input != "" {
		if err := json.Unmarshal([]byte(input), &in); err != nil {
			return encode(envelope{Error: "bad input: " + err.Error()})
		}
	}
	return encode(envelope{Ok: true, Data: f(in)})
}

func argString(args []js.Value) string {
	if len(args) == 0 || args[0].Type() != js.TypeString {
		return ""
	}
	return args[0].String()
}

// syncFunc registers a scenario that returns at once.
func syncFunc[In, Out any](f func(In) Out) js.Func {
	return js.FuncOf(func(_ js.Value, args []js.Value) any {
		return call(argString(args), f)
	})
}

// asyncFunc registers a scenario that waits for timers. It returns a Promise,
// because a blocking call in a js.Func would stop the browser event loop.
// An optional second argument is a JS function that gets each timeline
// event as a JSON string when it happens.
func asyncFunc[In, Out any](f func(context.Context, In) Out) js.Func {
	return js.FuncOf(func(_ js.Value, args []js.Value) any {
		input := argString(args)
		var onEvent js.Value
		if len(args) > 1 && args[1].Type() == js.TypeFunction {
			onEvent = args[1]
		}
		executor := js.FuncOf(func(_ js.Value, pargs []js.Value) any {
			resolve := pargs[0]
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if onEvent.Truthy() {
					ctx = scenarios.WithEventHook(ctx, func(e scenarios.Event) {
						defer func() { _ = recover() }() // a JS error must not stop the run
						b, _ := json.Marshal(e)
						onEvent.Invoke(string(b))
					})
				}
				resolve.Invoke(call(input, func(in In) Out { return f(ctx, in) }))
			}()
			return nil
		})
		defer executor.Release()
		return js.Global().Get("Promise").New(executor)
	})
}

func main() {
	api := js.Global().Get("Object").New()
	api.Set("version", runtime.Version())
	api.Set("option", syncFunc(scenarios.RunOption))
	api.Set("result", syncFunc(scenarios.RunResult))
	api.Set("validation", syncFunc(scenarios.RunValidation))
	api.Set("either", syncFunc(scenarios.RunEither))
	api.Set("patch", syncFunc(scenarios.RunPatch))
	api.Set("do", syncFunc(scenarios.RunDo))
	api.Set("tasks", asyncFunc(scenarios.RunTasks))
	api.Set("retry", asyncFunc(scenarios.RunRetry))
	js.Global().Set("monadz", api)

	if ready := js.Global().Get("onMonadzReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	select {} // keep the functions alive
}
