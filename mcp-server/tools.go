// Tool registration for the Gameplane MCP server. Every tool here is
// List/Get-shaped (or, for propose_fix, pure text generation) — see the
// package doc comment in main.go for why that's a hard invariant, not a
// convention. readOnlyTool() below is the one place a tool gets built, so
// there's a single spot to audit.
package main

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registeredToolNames lists every tool this server installs, in
// registration order. main_test.go uses it to assert the read-only
// invariant (no name/description implying a mutating verb) without needing
// a live server round-trip for that specific check.
var registeredToolNames = []string{
	"list_gameplane_resources",
	"get_gameplane_resource",
	"list_pods",
	"get_pod",
	"list_events",
	"get_pod_logs",
	"propose_fix",
}

// readOnlyTool builds a *mcp.Tool with ReadOnlyHint set, so any MCP client
// sees the read-only guarantee in the tool's own advertised annotations, not
// just in this server's implementation.
func readOnlyTool(name, description string) *mcp.Tool {
	destructive := false
	openWorld := false
	return &mcp.Tool{
		Name:        name,
		Description: description,
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    true,
			DestructiveHint: &destructive,
			IdempotentHint:  true,
			OpenWorldHint:   &openWorld,
		},
	}
}

// registerTools installs every tool on s, backed by c.
func registerTools(s *mcp.Server, c *Client) {
	mcp.AddTool(s, readOnlyTool(
		"list_gameplane_resources",
		"List objects of one Gameplane CRD kind (GameServer, GameTemplate, Backup, "+
			"BackupSchedule, Restore, Module, or ModuleSource). Read-only: get/list/watch "+
			"only, never create/update/delete.",
	), listResourcesHandler(c))

	mcp.AddTool(s, readOnlyTool(
		"get_gameplane_resource",
		"Fetch a single Gameplane CRD object by kind, namespace, and name. Read-only.",
	), getResourceHandler(c))

	mcp.AddTool(s, readOnlyTool(
		"list_pods",
		"List core Pods in a namespace (or all namespaces), optionally filtered by a "+
			"label selector. Read-only.",
	), listPodsHandler(c))

	mcp.AddTool(s, readOnlyTool(
		"get_pod",
		"Fetch a single Pod's spec and status by namespace and name. Read-only.",
	), getPodHandler(c))

	mcp.AddTool(s, readOnlyTool(
		"list_events",
		"List core Events in a namespace (or all namespaces), optionally filtered by a "+
			"field selector (e.g. \"involvedObject.name=my-server\") or label selector. Read-only.",
	), listEventsHandler(c))

	mcp.AddTool(s, readOnlyTool(
		"get_pod_logs",
		"Fetch a bounded tail of a Pod's container logs. Read-only: this only streams "+
			"existing log output, it never execs into or modifies the pod.",
	), getPodLogsHandler(c))

	mcp.AddTool(s, readOnlyTool(
		"propose_fix",
		"Given a resource reference and a symptom description, return SUGGESTED YAML "+
			"and/or kubectl commands as plain text for a human operator to review. This "+
			"tool never applies anything itself — it only reads cluster state (best "+
			"effort) to ground its suggestion and returns text.",
	), proposeFixHandler(c))
}

// --- list_gameplane_resources ---

type listResourcesInput struct {
	Kind          string `json:"kind" jsonschema:"Gameplane CRD kind to list: GameServer, GameTemplate, Backup, BackupSchedule, Restore, Module, or ModuleSource."`
	Namespace     string `json:"namespace,omitempty" jsonschema:"Namespace to list within. Ignored for cluster-scoped kinds (GameTemplate, Module, ModuleSource). Empty lists across all namespaces for namespaced kinds."`
	LabelSelector string `json:"labelSelector,omitempty" jsonschema:"Optional Kubernetes label selector, e.g. 'gameplane.local/template=minecraft-java'."`
}

func listResourcesHandler(c *Client) mcp.ToolHandlerFor[listResourcesInput, any] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in listResourcesInput) (*mcp.CallToolResult, any, error) {
		list, err := c.ListCRD(ctx, in.Kind, in.Namespace, in.LabelSelector)
		if err != nil {
			return nil, nil, err
		}
		text, err := marshalIndent(list)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal %s list: %w", in.Kind, err)
		}
		return textResult(text), nil, nil
	}
}

// --- get_gameplane_resource ---

type getResourceInput struct {
	Kind      string `json:"kind" jsonschema:"Gameplane CRD kind: GameServer, GameTemplate, Backup, BackupSchedule, Restore, Module, or ModuleSource."`
	Namespace string `json:"namespace,omitempty" jsonschema:"Namespace containing the object. Required for namespaced kinds (GameServer, Backup, BackupSchedule, Restore); ignored for cluster-scoped kinds."`
	Name      string `json:"name" jsonschema:"Object name."`
}

func getResourceHandler(c *Client) mcp.ToolHandlerFor[getResourceInput, any] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in getResourceInput) (*mcp.CallToolResult, any, error) {
		obj, err := c.GetCRD(ctx, in.Kind, in.Namespace, in.Name)
		if err != nil {
			return nil, nil, err
		}
		text, err := marshalIndent(obj)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal %s %s/%s: %w", in.Kind, in.Namespace, in.Name, err)
		}
		return textResult(text), nil, nil
	}
}

// --- list_pods ---

type listPodsInput struct {
	Namespace     string `json:"namespace,omitempty" jsonschema:"Namespace to list within. Empty lists across all namespaces."`
	LabelSelector string `json:"labelSelector,omitempty" jsonschema:"Optional Kubernetes label selector, e.g. 'gameplane.local/server=my-server'."`
}

func listPodsHandler(c *Client) mcp.ToolHandlerFor[listPodsInput, any] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in listPodsInput) (*mcp.CallToolResult, any, error) {
		list, err := c.ListPods(ctx, in.Namespace, in.LabelSelector)
		if err != nil {
			return nil, nil, err
		}
		text, err := marshalIndent(list)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal pod list: %w", err)
		}
		return textResult(text), nil, nil
	}
}

// --- get_pod ---

type getPodInput struct {
	Namespace string `json:"namespace" jsonschema:"Namespace containing the pod."`
	Name      string `json:"name" jsonschema:"Pod name."`
}

func getPodHandler(c *Client) mcp.ToolHandlerFor[getPodInput, any] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in getPodInput) (*mcp.CallToolResult, any, error) {
		pod, err := c.GetPod(ctx, in.Namespace, in.Name)
		if err != nil {
			return nil, nil, err
		}
		text, err := marshalIndent(pod)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal pod %s/%s: %w", in.Namespace, in.Name, err)
		}
		return textResult(text), nil, nil
	}
}

// --- list_events ---

type listEventsInput struct {
	Namespace     string `json:"namespace,omitempty" jsonschema:"Namespace to list within. Empty lists across all namespaces."`
	FieldSelector string `json:"fieldSelector,omitempty" jsonschema:"Optional field selector, e.g. 'involvedObject.name=my-server,involvedObject.kind=GameServer'."`
	LabelSelector string `json:"labelSelector,omitempty" jsonschema:"Optional label selector."`
}

func listEventsHandler(c *Client) mcp.ToolHandlerFor[listEventsInput, any] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in listEventsInput) (*mcp.CallToolResult, any, error) {
		list, err := c.ListEvents(ctx, in.Namespace, in.FieldSelector)
		if err != nil {
			return nil, nil, err
		}
		if in.LabelSelector != "" {
			list = filterEventsByLabel(list, in.LabelSelector)
		}
		text, err := marshalIndent(list)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal event list: %w", err)
		}
		return textResult(text), nil, nil
	}
}

// filterEventsByLabel is a client-side fallback: corev1.Event doesn't carry
// its involved object's labels, and the API server's LabelSelector on
// /events only matches the Event object's own (rarely-set) labels. Filtering
// here on the Event's own labels keeps the behavior honest about what
// labelSelector actually matches, without pretending to filter by the
// involved object's labels.
func filterEventsByLabel(list *corev1.EventList, selector string) *corev1.EventList {
	sel, err := labels.Parse(selector)
	if err != nil {
		return list
	}
	filtered := make([]corev1.Event, 0, len(list.Items))
	for _, ev := range list.Items {
		if sel.Matches(labels.Set(ev.Labels)) {
			filtered = append(filtered, ev)
		}
	}
	out := list.DeepCopy()
	out.Items = filtered
	return out
}

// --- get_pod_logs ---

type getPodLogsInput struct {
	Namespace string `json:"namespace" jsonschema:"Namespace containing the pod."`
	Pod       string `json:"pod" jsonschema:"Pod name."`
	Container string `json:"container,omitempty" jsonschema:"Container name. Required if the pod has more than one container."`
	TailLines int64  `json:"tailLines,omitempty" jsonschema:"Number of lines to return from the end of the log, capped at 5000. Defaults to 200."`
	Previous  bool   `json:"previous,omitempty" jsonschema:"If true, fetch logs from the previous (crashed) instance of the container instead of the current one."`
}

func getPodLogsHandler(c *Client) mcp.ToolHandlerFor[getPodLogsInput, any] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in getPodLogsInput) (*mcp.CallToolResult, any, error) {
		logs, err := c.PodLogs(ctx, in.Namespace, in.Pod, in.Container, in.TailLines, in.Previous)
		if err != nil {
			return nil, nil, err
		}
		return textResult(logs), nil, nil
	}
}

// --- propose_fix ---

type proposeFixInput struct {
	Kind      string `json:"kind,omitempty" jsonschema:"Optional: the Gameplane CRD kind the symptom concerns (GameServer, Backup, BackupSchedule, Restore, GameTemplate, Module, ModuleSource), or 'Pod'."`
	Namespace string `json:"namespace,omitempty" jsonschema:"Optional: namespace of the resource the symptom concerns."`
	Name      string `json:"name,omitempty" jsonschema:"Optional: name of the resource the symptom concerns."`
	Symptom   string `json:"symptom" jsonschema:"Required. Free-text description of the observed problem, e.g. 'GameServer stuck in Pending for 10 minutes' or 'CrashLoopBackOff on the agent container'."`
}

func proposeFixHandler(c *Client) mcp.ToolHandlerFor[proposeFixInput, any] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in proposeFixInput) (*mcp.CallToolResult, any, error) {
		text := buildFixSuggestion(ctx, c, in)
		return textResult(text), nil, nil
	}
}

// --- shared helpers ---

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

func marshalIndent(v any) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
