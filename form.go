package eui

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
)

// TextValidator returns an inline error, or an empty string for a valid value.
// Validators run synchronously on the UI goroutine. They must not mutate UI or
// model state. Values are committed text, never IME preedit.
type TextValidator func(string) string

// Required rejects empty and whitespace-only text without trimming stored values.
func Required(value string) string {
	if strings.TrimSpace(value) == "" {
		return "This field is required."
	}
	return ""
}

// NumberRange accepts finite decimal/scientific numbers within inclusive bounds.
// Empty text is optional; combine with Required when needed. Invalid bounds panic.
func NumberRange(minimum, maximum float64) TextValidator {
	if math.IsNaN(minimum) || math.IsNaN(maximum) || math.IsInf(minimum, 0) || math.IsInf(maximum, 0) || minimum > maximum {
		panic("eui: invalid number validation bounds")
	}
	return func(value string) string {
		value = strings.TrimSpace(value)
		if value == "" {
			return ""
		}
		n, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return "Enter a finite number."
		}
		if n < minimum || n > maximum {
			return fmt.Sprintf("Enter a number from %g to %g.", minimum, maximum)
		}
		return ""
	}
}

// IntegerRange accepts base-10 int64 values within inclusive bounds. Empty text
// is optional; combine with Required. Invalid bounds panic.
func IntegerRange(minimum, maximum int64) TextValidator {
	if minimum > maximum {
		panic("eui: invalid integer validation bounds")
	}
	return func(value string) string {
		value = strings.TrimSpace(value)
		if value == "" {
			return ""
		}
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return "Enter a whole number."
		}
		if n < minimum || n > maximum {
			return fmt.Sprintf("Enter a whole number from %d to %d.", minimum, maximum)
		}
		return ""
	}
}

// FormField composes an ordinary input with an inline error label. Its initial
// value is a draft, not a binding to an application variable. It can belong to
// one Form. Configure its input before placing it in a window.
type FormField struct {
	ctx        *Context
	name       string
	input      *ItemData
	view       *ItemData
	errorLabel *ItemData
	validators []TextValidator
	message    string
	validated  bool
	form       *Form
}

// NewFormField creates a named single-line field. Names must be nonempty and
// unique within a form. Validation is revealed by Validate/Apply, then updates
// on committed user edits, undo/redo, ReplaceText and UpdateText. Assigning Text
// directly requires an explicit Validate. Handler remains available to the host.
func (ctx *Context) NewFormField(name, label, value string, validators ...TextValidator) *FormField {
	ctx.mustAlive()
	if strings.TrimSpace(name) == "" {
		panic("eui: form field name is required")
	}
	input, _ := ctx.NewInput()
	input.Label = label
	input.Position = Point{}
	input.FillWidth = true
	input.UpdateText(value)
	message := ctx.NewWrappedLabel("", 300)
	message.Position = Point{}
	message.FillWidth = true
	message.Invisible = true
	view := ctx.NewColumn(input, message)
	view.FillWidth = true
	view.Gap = 4
	field := &FormField{ctx: ctx, name: name, input: input, view: view, errorLabel: message, validators: slices.Clone(validators)}
	input.formField = field
	return field
}

func (field *FormField) Name() string          { field.ctx.mustAlive(); return field.name }
func (field *FormField) Input() *ItemData      { field.ctx.mustAlive(); return field.input }
func (field *FormField) View() *ItemData       { field.ctx.mustAlive(); return field.view }
func (field *FormField) ErrorLabel() *ItemData { field.ctx.mustAlive(); return field.errorLabel }
func (field *FormField) Error() string         { field.ctx.mustAlive(); return field.message }

// Validate reveals this field's first error. An unfinished composition blocks
// validation without committing or discarding preedit. All fields are validated,
// including disabled/hidden fields; use optional/custom rules for conditional data.
func (field *FormField) Validate() bool {
	field.ctx.mustAlive()
	field.validated = true
	message := ""
	if field.input.IsComposing() {
		message = "Finish composing this value."
	} else {
		value := field.input.editText()
		for _, validator := range field.validators {
			if validator != nil {
				if message = validator(value); message != "" {
					break
				}
			}
		}
	}
	field.setError(message)
	return message == ""
}

func (field *FormField) setError(message string) {
	if field.message == message {
		return
	}
	field.message = message
	caption := ""
	if message != "" {
		caption = "Error: " + message
	}
	field.errorLabel.UpdateText(caption)
	field.errorLabel.SetVisible(message != "")
}

func (item *itemData) validateFormEdit() {
	if field := item.formField; field != nil && field.validated {
		field.Validate()
	}
}

// Form owns a set of text drafts and a separate applied snapshot. It does not
// write application variables. Copy Values into the model only after Apply
// succeeds. All methods belong to the context's UI goroutine.
type Form struct {
	ctx     *Context
	fields  []*FormField
	view    *ItemData
	applied map[string]string
}

// NewForm arranges fields in a column and snapshots their current committed
// text as the initial applied values. Foreign, duplicate or reused fields panic
// before any field is attached. Initial values are not assumed to be valid.
func (ctx *Context) NewForm(fields ...*FormField) *Form {
	ctx.mustAlive()
	seen := make(map[string]bool, len(fields))
	for _, field := range fields {
		if field == nil || field.ctx != ctx || field.form != nil || seen[field.name] || field.view.Parent != nil || field.view.ParentWindow != nil {
			panic("eui: form requires unattached, uniquely named fields from its context")
		}
		seen[field.name] = true
	}
	form := &Form{ctx: ctx, fields: slices.Clone(fields), view: ctx.NewColumn()}
	form.view.FillWidth, form.view.Gap = true, 12
	for _, field := range fields {
		field.form = form
		form.view.AddItem(field.view)
	}
	form.applied = form.Draft()
	return form
}

func (form *Form) View() *ItemData      { form.ctx.mustAlive(); return form.view }
func (form *Form) Fields() []*FormField { form.ctx.mustAlive(); return slices.Clone(form.fields) }

// Draft returns a copy of the current committed input text, excluding preedit.
// Draft and Values include logical password values when inputs use HideText.
func (form *Form) Draft() map[string]string {
	form.ctx.mustAlive()
	values := make(map[string]string, len(form.fields))
	for _, field := range form.fields {
		values[field.name] = field.input.editText()
	}
	return values
}

// Values returns a copy of the last successfully applied (or initial) values.
func (form *Form) Values() map[string]string { form.ctx.mustAlive(); return maps.Clone(form.applied) }

func (form *Form) Dirty() bool {
	form.ctx.mustAlive()
	for _, field := range form.fields {
		if field.input.IsComposing() || field.input.editText() != form.applied[field.name] {
			return true
		}
	}
	return false
}

// Validate reveals all errors and focuses the first available invalid field.
// An active composition retains focus so validation cannot discard preedit.
func (form *Form) Validate() bool {
	form.ctx.mustAlive()
	valid, composing := true, false
	var first *ItemData
	for _, field := range form.fields {
		composing = composing || field.input.IsComposing()
		if !field.Validate() {
			valid = false
			if first == nil && focusAvailable(field.input) {
				first = field.input
			}
		}
	}
	if first != nil && !composing && form.ctx.FocusedItem() != first {
		form.ctx.Focus(first)
	}
	return valid
}

// Errors returns a copy of the currently revealed errors, keyed by field name.
func (form *Form) Errors() map[string]string {
	form.ctx.mustAlive()
	errors := make(map[string]string)
	for _, field := range form.fields {
		if field.message != "" {
			errors[field.name] = field.message
		}
	}
	return errors
}

// FormValidationError reports failed fields without retaining any draft values.
type FormValidationError struct{ Fields map[string]string }

func (err *FormValidationError) Error() string {
	return fmt.Sprintf("eui: %d form field(s) failed validation", len(err.Fields))
}

// Apply validates every field before atomically replacing the applied snapshot.
// Failure leaves Values unchanged. Success keeps the input's normal undo history.
func (form *Form) Apply() error {
	if !form.Validate() {
		return &FormValidationError{Fields: form.Errors()}
	}
	form.applied = form.Draft()
	return nil
}

// Cancel restores the last applied values, discards preedit and undo/redo, and
// hides errors. It emits no change events and does not modify application data.
func (form *Form) Cancel() {
	form.ctx.mustAlive()
	for _, field := range form.fields {
		field.validated = false
		field.input.CancelComposition()
		field.input.UpdateText(form.applied[field.name])
		field.input.textEdit = nil
		field.input.editor()
		field.input.markDirty()
		field.setError("")
	}
}
