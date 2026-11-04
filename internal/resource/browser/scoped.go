package browser

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/tomatool/tomato/internal/resource"
)

var errRoleAbsent = errors.New("named role is absent")

// locateRole uses computed roles and names, including implicit HTML semantics.
// It deliberately has no text fallback: a heading called Review is not a dialog.
func locateRole(ctx context.Context, role, name string) (runtime.RemoteObjectID, error) {
	doc, err := document(ctx)
	if err != nil {
		return "", err
	}
	tree, err := chromedp.Call(ctx, accessibility.QueryAXTree, accessibility.QueryAXTreeParams{ObjectID: doc, Role: role, AccessibleName: normalize(name)})
	if err != nil {
		return "", fmt.Errorf("reading the accessibility tree: %w", err)
	}
	args := []*runtime.CallArgument{jsArg("namedRole"), jsArg(role), jsArg(name)}
	seen := map[int64]bool{}
	for _, n := range tree.Nodes {
		if n.Role == nil || jsonString(n.Role.Value) != role || n.Name == nil || normalize(jsonString(n.Name.Value)) != normalize(name) {
			continue
		}
		if n.Ignored || n.BackendDOMNodeID == 0 || seen[int64(n.BackendDOMNodeID)] {
			continue
		}
		seen[int64(n.BackendDOMNodeID)] = true
		node, err := chromedp.Call(ctx, dom.ResolveNode, dom.ResolveNodeParams{BackendNodeID: n.BackendDOMNodeID})
		if err != nil || node.Object == nil {
			continue
		}
		args = append(args, &runtime.CallArgument{ObjectID: node.Object.ObjectID})
	}
	result, err := chromedp.Call(ctx, runtime.CallFunctionOn, runtime.CallFunctionOnParams{FunctionDeclaration: pageJS, ObjectID: doc, Arguments: args})
	if err != nil {
		return "", err
	}
	if result.ExceptionDetails != nil {
		return "", &chromedp.ExceptionError{ExceptionDetails: result.ExceptionDetails}
	}
	if result.Result == nil {
		return "", errors.New("locating a named role returned nothing")
	}
	if result.Result.Type == runtime.RemoteObjectTypeString {
		message := jsonString(result.Result.Value)
		if message == "absent" {
			return "", fmt.Errorf("no visible %s named %q: %w", role, name, errRoleAbsent)
		}
		return "", errors.New(message)
	}
	return result.Result.ObjectID, nil
}

func (r *Browser) seesRoleWithin(role, name, within string) error {
	role, name = resource.ReplaceVariables(role), resource.ReplaceVariables(name)
	wait, err := parseWait(within)
	if err != nil {
		return err
	}
	ctx, cancel, err := r.stepContext(wait)
	if err != nil {
		return err
	}
	defer cancel()
	err = retry(ctx, wait, func() error { _, err := locateRole(ctx, role, name); return err })
	if err != nil {
		return fmt.Errorf("%q did not see a %s named %q within %s: %w", r.name, role, name, within, err)
	}
	return nil
}

func (r *Browser) noLongerSeesRole(role, name, within string) error {
	role, name = resource.ReplaceVariables(role), resource.ReplaceVariables(name)
	wait, err := parseWait(within)
	if err != nil {
		return err
	}
	ctx, cancel, err := r.stepContext(wait)
	if err != nil {
		return err
	}
	defer cancel()
	err = retry(ctx, wait, func() error {
		_, err := locateRole(ctx, role, name)
		if errors.Is(err, errRoleAbsent) {
			return nil
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("still visible")
	})
	if err != nil {
		return fmt.Errorf("%q still sees a %s named %q after %s: %w", r.name, role, name, within, err)
	}
	return nil
}

func (r *Browser) seesInRoleWithin(text, role, name, within string) error {
	text, role, name = resource.ReplaceVariables(text), resource.ReplaceVariables(role), resource.ReplaceVariables(name)
	wait, err := parseWait(within)
	if err != nil {
		return err
	}
	ctx, cancel, err := r.stepContext(wait)
	if err != nil {
		return err
	}
	defer cancel()
	var shown string
	err = retry(ctx, wait, func() error {
		scope, err := locateRole(ctx, role, name)
		if err != nil {
			return err
		}
		shown, err = call[string](ctx, scope, "text")
		if err != nil {
			return err
		}
		if !strings.Contains(shown, normalize(text)) {
			return errors.New("text is not shown in the named region")
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("%q did not see %q in the %s named %q within %s; the region shows %q: %w", r.name, text, role, name, within, excerpt(shown), err)
	}
	return nil
}

func (r *Browser) doesNotSeeInRole(text, role, name string) error {
	text, role, name = resource.ReplaceVariables(text), resource.ReplaceVariables(role), resource.ReplaceVariables(name)
	ctx, cancel, err := r.stepContext(r.timeout)
	if err != nil {
		return err
	}
	defer cancel()
	scope, err := locateRole(ctx, role, name)
	if err != nil {
		return err
	}
	shown, err := call[string](ctx, scope, "text")
	if err != nil {
		return err
	}
	if strings.Contains(shown, normalize(text)) {
		return fmt.Errorf("%q sees %q in the %s named %q, expected the region not to show it", r.name, text, role, name)
	}
	return nil
}
