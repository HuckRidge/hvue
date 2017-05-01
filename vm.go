package hvue

import (
	"reflect"

	"github.com/gopherjs/gopherjs/js"
)

type VM struct {
	*js.Object
}

var vmType = reflect.TypeOf(&VM{})

// NewVM returns a new vm, analogous to Javascript `new Vue(...)`.  See
// https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis and
// https://commandcenter.blogspot.com.au/2014/01/self-referential-functions-and-design.html
// for discussions of how the options work, and also see the examples tree.
//
// If you use a data object (via DataS) and it has a VM field, it's set to
// this new VM.
func NewVM(opts ...option) *VM {
	c := &Config{Object: NewObject()}
	c.Option(opts...)
	vm := &VM{Object: js.Global.Get("Vue").New(c)}
	if c.dataValue.IsValid() {
		if vmField := c.dataValue.FieldByName("VM"); vmField.IsValid() {
			vmField.Set(reflect.ValueOf(vm))
		}
	}
	return vm
}

// El sets the vm's el slot.
func El(selector string) option {
	return func(c *Config) {
		c.El = selector
	}
}

// Data sets a single data field.  Data can be called multiple times for the
// same vm.
func Data(name string, value interface{}) option {
	return func(c *Config) {
		if c.Data == js.Undefined {
			c.Data = NewObject()
		}
		c.Data.Set(name, value)
	}
}

// DataS sets the struct `value` as the entire contents of the vm's data
// field.  `value` should be a pointer to the struct.  If the object has a VM
// field, NewVM sets it to the new VM object.
func DataS(value interface{}) option {
	return func(c *Config) {
		if c.Data != js.Undefined {
			panic("Cannot use hvue.Data and hvue.DataS together")
			c.Data = NewObject()
		}
		c.Object.Set("data", value)
		c.dataValue = reflect.ValueOf(value).Elem()
	}
}

// MethodsOf sets up vm.methods with the exported methods of the type that t
// is an instance of.  Call it like MethodsOf(&SomeType{}).  SomeType must be
// a pure Javascript object, with no Go fields.  That is, all slots just have
// `js:"..."` tags.
func MethodsOf(t interface{}) option {
	return func(c *Config) {
		if c.Methods == js.Undefined {
			c.Methods = NewObject()
		}
		// Get the type of t
		typ := reflect.TypeOf(t)

		if typ.Kind() != reflect.Ptr {
			panic("Item passed to MethodsOf must be a pointer")
		}

		// Create a new receiver.  "Same" receiver used for all methods, with
		// its Object slot set differently(?) each time.  typ is a pointer type
		// so you have to get the type of the thing it points to with Elem() and
		// create a new one of those.
		receiver := reflect.New(typ.Elem())

		// Loop through all methods of the type
		for i := 0; i < typ.NumMethod(); i++ {
			// Get the i'th method's reflect.Method
			m := typ.Method(i)

			c.Methods.Set(m.Name,
				js.MakeFunc(
					func(this *js.Object, jsArgs []*js.Object) interface{} {
						// Set the receiver's Object slot to c.Data.  receiver is a
						// pointer so you have to dereference it with Elem().
						receiver.Elem().Field(0).Set(reflect.ValueOf(c.Data))

						// Construct the arglist
						goArgs := make([]reflect.Value, m.Type.NumIn())
						goArgs[0] = receiver
						i := 1

						// If the 2nd arg (the *first* arg if you don't include the
						// receiver) expects a *VM, pass `this`.
						if m.Type.NumIn() > 1 && m.Type.In(1) == vmType {
							vm := &VM{Object: this}
							goArgs[1] = reflect.ValueOf(vm)
							i = 2
						}

						for j := 0; j < len(jsArgs); i, j = i+1, j+1 {
							goArgs[i] = reflect.ValueOf(jsArgs[j])
						}

						result := m.Func.Call(goArgs)

						// I don't think method results are ever actually used, but
						// I could be wrong.
						if len(result) >= 1 {
							return result[0].Interface()
						}
						return nil
					}))
		}
	}
}
