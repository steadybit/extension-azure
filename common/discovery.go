/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package common

import (
	"context"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
)

// DiscoverViaResourceGraph runs an Azure Resource Graph query, iterates the
// result rows, and converts each map[string]any to a Target via toTarget.
// Returns an empty slice (not nil) on success with no rows so callers can
// safely range over the result.
//
// The query is scoped to AZURE_SUBSCRIPTION_ID if that env var is set; with no
// env var, Resource Graph runs across every subscription the SP can see.
func DiscoverViaResourceGraph(
	ctx context.Context,
	client ArmResourceGraphApi,
	query string,
	toTarget func(map[string]any) discovery_kit_api.Target,
) ([]discovery_kit_api.Target, error) {
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{&subscriptionId}
	}
	res, err := client.Resources(ctx, armresourcegraph.QueryRequest{
		Query: new(query),
		Options: &armresourcegraph.QueryRequestOptions{
			ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
		},
		Subscriptions: subscriptions,
	}, nil)
	if err != nil {
		return nil, err
	}
	targets := make([]discovery_kit_api.Target, 0)
	rows, ok := res.Data.([]any)
	if !ok {
		return targets, nil
	}
	for _, r := range rows {
		if items, ok := r.(map[string]any); ok {
			targets = append(targets, toTarget(items))
		}
	}
	return targets, nil
}

// StringFromMap returns m[key] as a string, or "" if absent or not a string.
func StringFromMap(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// StringSliceFromMap returns m[key] as []string. Returns nil if the key is
// absent or not a slice. Non-string and empty-string entries are skipped.
func StringSliceFromMap(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}
