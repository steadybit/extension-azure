/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extcosmosdb

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-azure/common"
	"github.com/steadybit/extension-azure/config"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"os"
)

const (
	TargetIDCosmosDbAccount = "com.steadybit.extension_azure.cosmosdb.account"
	targetIcon              = "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjQiIGhlaWdodD0iMjQiIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZD0iTTUuMjQyMjkgNy44MDQ2OEM1LjIxNjQgNy44MDQ4MSA1LjE5MDc2IDcuNzk5NzkgNS4xNjY3OSA3Ljc4OTk5QzUuMTQyODIgNy43ODAwNyA1LjEyMTE1IDcuNzY1NTEgNS4xMDI4MiA3Ljc0NzIxQzUuMDg0NDkgNy43Mjg3OSA1LjA3MDAxIDcuNzA3MDEgNS4wNjAyNyA3LjY4MjkxQzUuMDUwNCA3LjY1ODgyIDUuMDQ1NCA3LjYzMzA1IDUuMDQ1NjUgNy42MDcwMkM1LjA0NDc2IDcuMDA4NjIgNC44MDc4NyA2LjQzNTA5IDQuMzg3MDQgNi4wMTE5M0MzLjk2NjA4IDUuNTg4NzggMy4zOTU1MyA1LjM1MDc4IDIuODAwMjQgNS4zNDk4OEMyLjc0ODMyIDUuMzQ5ODggMi42OTg0NiA1LjMyOTE0IDIuNjYxNTQgNS4yOTI0MUMyLjYyNDYzIDUuMjU1NTYgMi42MDM4NiA1LjIwNTU3IDIuNjAzNDcgNS4xNTMzOEMyLjYwMzQ3IDUuMTAwOTQgMi42MjQyNCA1LjA1MDY4IDIuNjYxMDMgNS4wMTM1N0MyLjY5Nzk1IDQuOTc2NDYgMi43NDc5NCA0Ljk1NTcyIDIuODAwMTEgNC45NTU3MkMzLjM5NTUzIDQuOTU0ODIgMy45NjYzNCA0LjcxNjU2IDQuMzg3MyA0LjI5MzI4QzQuODA4MjUgMy44Njk5OSA1LjA0NTAxIDMuMjk2MDggNS4wNDU1MyAyLjY5NzU1QzUuMDQ1NCAyLjY3MTUyIDUuMDUwNCAyLjY0NTc1IDUuMDYwMTQgMi42MjE2NUM1LjA3MDAxIDIuNTk3NTYgNS4wODQ0OSAyLjU3NTc4IDUuMTAyNyAyLjU1NzM2QzUuMTIxMDMgMi41Mzg5MyA1LjE0MjY5IDIuNTI0MzcgNS4xNjY2NiAyLjUxNDU4QzUuMTkwNjMgMi41MDQ2NiA1LjIxNjI3IDIuNDk5NjMgNS4yNDIxNiAyLjQ5OTg5QzUuMjY4MDUgMi40OTk3NiA1LjI5MzY5IDIuNTA0NzggNS4zMTc2NiAyLjUxNDU4QzUuMzQxNjMgMi41MjQ1IDUuMzYzMyAyLjUzOTA2IDUuMzgxNjMgMi41NTczNkM1LjM5OTk2IDIuNTc1NzggNS40MTQ0NCAyLjU5NzU2IDUuNDI0MTggMi42MjE2NUM1LjQzNDA1IDIuNjQ1NzUgNS40MzkwNSAyLjY3MTUyIDUuNDM4OCAyLjY5NzU1QzUuNDM5NjkgMy4yOTU5NSA1LjY3NjU4IDMuODY5NDggNi4wOTc0MSA0LjI5MjYzQzYuNTE4MzcgNC43MTU3OSA3LjA4ODkyIDQuOTUzNzggNy42ODQyMSA0Ljk1NDY5QzcuNzEwMSA0Ljk1NDU2IDcuNzM1NzQgNC45NTk1OCA3Ljc1OTcxIDQuOTY5MzhDNy43ODM2OCA0Ljk3OTMgNy44MDUzNSA0Ljk5Mzg2IDcuODIzNjggNS4wMTIxNkM3Ljg0MjAxIDUuMDMwNTggNy44NTY0OSA1LjA1MjM2IDcuODY2MjMgNS4wNzY0NUM3Ljg3NjEgNS4xMDA1NSA3Ljg4MTEgNS4xMjYzMiA3Ljg4MDg1IDUuMTUyMzVDNy44ODA5NyA1LjE3ODM4IDcuODc1OTggNS4yMDQxNSA3Ljg2NjIzIDUuMjI4MjRDNy44NTYzNiA1LjI1MjM0IDcuODQxODggNS4yNzQxMiA3LjgyMzY4IDUuMjkyNTRDNy44MDUzNSA1LjMxMDk3IDcuNzgzNjggNS4zMjU1MyA3Ljc1OTcxIDUuMzM1MzJDNy43MzU3NCA1LjM0NTI0IDcuNzEwMSA1LjM1MDI3IDcuNjg0MjEgNS4zNTAwMUM3LjA4ODkyIDUuMzUwNjYgNi41MTgxMSA1LjU4ODY1IDYuMDk3MTUgNi4wMTE4MUM1LjY3NjE5IDYuNDM0OTYgNS40Mzk0NCA3LjAwODc1IDUuNDM4OCA3LjYwNzE1QzUuNDM4NTQgNy42NTk0NiA1LjQxNzY1IDcuNzA5NTkgNS4zODA4NiA3Ljc0NjdDNS4zNDQwNyA3Ljc4MzY4IDUuMjk0NDYgNy44MDQ0MiA1LjI0MjI5IDcuODA0NjhaIiBmaWxsPSJjdXJyZW50Q29sb3IiLz4KPHBhdGggZD0iTTE5LjM4ODMgMjEuNTAwMUMxOS4zNDY3IDIxLjUwMDEgMTkuMzA2NiAyMS40ODM1IDE5LjI3NyAyMS40NTM4QzE5LjI0NzUgMjEuNDI0MSAxOS4yMzA4IDIxLjM4MzggMTkuMjMwOCAyMS4zNDE4QzE5LjIzMDMgMjAuODYzMiAxOS4wNDA4IDIwLjQwNDIgMTguNzA0MiAyMC4wNjU5QzE4LjM2NzUgMTkuNzI3NCAxNy45MTEgMTkuNTM2OSAxNy40MzQ4IDE5LjUzNjRDMTcuMzkzMyAxOS41MzY0IDE3LjM1MzMgMTkuNTE5OSAxNy4zMjM4IDE5LjQ5MDRDMTcuMjk0MyAxOS40NjA5IDE3LjI3NzUgMTkuNDIwOCAxNy4yNzcxIDE5LjM3OTFDMTcuMjc3MSAxOS4zMzcxIDE3LjI5MzggMTkuMjk2OSAxNy4zMjMzIDE5LjI2NzFDMTcuMzUyOSAxOS4yMzc1IDE3LjM5MyAxOS4yMjA3IDE3LjQzNDcgMTkuMjIwN0MxNy45MTEgMTkuMjIwNSAxOC4zNjc3IDE5LjAzMDEgMTguNzA0NSAxOC42OTE1QzE5LjA0MTIgMTguMzUzIDE5LjIzMDUgMTcuODkzOSAxOS4yMzA4IDE3LjQxNTJDMTkuMjMwOCAxNy4zNzMyIDE5LjI0NzUgMTcuMzMzIDE5LjI3NyAxNy4zMDMyQzE5LjMwNjYgMTcuMjczNiAxOS4zNDY3IDE3LjI1NjggMTkuMzg4MyAxNy4yNTY4QzE5LjQzMDEgMTcuMjU2OCAxOS40NzAzIDE3LjI3MzYgMTkuNDk5NyAxNy4zMDMyQzE5LjUyOTMgMTcuMzMzIDE5LjU0NTkgMTcuMzczMiAxOS41NDU5IDE3LjQxNTJDMTkuNTQ2MyAxNy44OTM5IDE5LjczNTYgMTguMzUzIDIwLjA3MjMgMTguNjkxNUMyMC40MDkxIDE5LjAzMDEgMjAuODY1OCAxOS4yMjA1IDIxLjM0MiAxOS4yMjA3QzIxLjM4MzggMTkuMjIwNyAyMS40MjM5IDE5LjIzNzUgMjEuNDUzNCAxOS4yNjcxQzIxLjQ4MyAxOS4yOTY5IDIxLjQ5OTYgMTkuMzM3MSAyMS40OTk2IDE5LjM3OTFDMjEuNDk5NiAxOS40MjExIDIxLjQ4MyAxOS40NjE0IDIxLjQ1MzQgMTkuNDkxQzIxLjQyMzkgMTkuNTIwOCAyMS4zODM4IDE5LjUzNzQgMjEuMzQyIDE5LjUzNzRDMjAuODY1OSAxOS41Mzc5IDIwLjQwOTUgMTkuNzI4NCAyMC4wNzI3IDIwLjA2NjlDMTkuNzM2MSAyMC40MDUzIDE5LjU0NjcgMjAuODY0MiAxOS41NDYgMjEuMzQyOEMxOS41NDU4IDIxLjM4NDYgMTkuNTI5IDIxLjQyNDYgMTkuNDk5NSAyMS40NTQxQzE5LjQ3IDIxLjQ4MzUgMTkuNDMwMSAyMS41MDAxIDE5LjM4ODMgMjEuNTAwMVoiIGZpbGw9ImN1cnJlbnRDb2xvciIvPgo8cGF0aCBkPSJNMTAuNDA1NiA1LjI0NDQ2QzEyLjE3NDkgNC44MTk2NyAxNC4wNDAzIDUuMTE5MjYgMTUuNTkwMSA2LjA3NjQ5QzE2LjE2NDkgNi40MzE1MiAxNi42Nzg2IDYuODY2NDEgMTcuMTE5NCA3LjM2MzZDMTcuMDMyNSA3LjMzMTIxIDE2Ljk0NDMgNy4zMDI4NyAxNi44NTM4IDcuMjgyNTVDMTYuNTYwMyA3LjE5ODMyIDE2LjI1MTIgNy4xODI1NCAxNS45NTA1IDcuMjM1NjdDMTUuNjQ5OCA3LjI4ODg5IDE1LjM2MzkgNy40MTAyOSAxNS4xMTY1IDcuNTkwMTdDMTQuODY5MyA3Ljc2OTkyIDE0LjY2NjEgOC4wMDM2NSAxNC41MjE4IDguMjczNzZDMTQuMzc3NCA4LjU0NDEzIDE0LjI5NSA4Ljg0NDE2IDE0LjI4MjUgOS4xNTA3MUMxNC4yMDgzIDkuMTQyMTEgMTQuMTMzNSA5LjEzNzA5IDE0LjA1ODkgOS4xMzYwN0MxMy42MDE1IDkuMTM2MTcgMTMuMTU4OSA5LjI5ODkyIDEyLjgwODkgOS41OTUwNUMxMi40NTg4IDkuODkxNTQgMTIuMjIzNSAxMC4zMDM2IDEyLjE0NTggMTAuNzU3MkMxMi4wNjgyIDExLjIxMDQgMTIuMTUyOSAxMS42NzY2IDEyLjM4NDEgMTIuMDczNkMxMi42MTUzIDEyLjQ3MDYgMTIuOTc4MiAxMi43NzMyIDEzLjQwOTUgMTIuOTI3MUMxMy42MDIgMTMuMDAzIDEzLjgwODEgMTMuMDQxMSAxNC4wMTQ5IDEzLjAzOTRIMTYuMjgzNUMxNy4xODg2IDEyLjM1MDYgMTguMDEyMSAxMS41NjAxIDE4Ljc0MDUgMTAuNjg0OUMxOC45Njc5IDExLjkwODMgMTguODYzMiAxMy4xNzM5IDE4LjQzMTkgMTQuMzQ1QzE3Ljk2MTEgMTUuNjIzNiAxNy4xMjMgMTYuNzMzNCAxNi4wMjQ3IDE3LjUzMzVDMTQuOTI2NSAxOC4zMzM2IDEzLjYxNjMgMTguNzg4MSAxMi4yNjEgMTguODQwMkMxMC45MDU2IDE4Ljg5MjIgOS41NjUzNSAxOC41MzgzIDguNDA5NDYgMTcuODI0NUM3Ljg2OTc3IDE3LjQ5MTMgNy4zODIwMSAxNy4wODY1IDYuOTU3MzEgMTYuNjIzNEg4LjMyMTU3QzguNTY1MzggMTYuNjI5NyA4LjgwODM2IDE2LjU4NzYgOS4wMzU0MyAxNi40OTg0QzkuMjYyNzEgMTYuNDA5MSA5LjQ3MDc0IDE2LjI3NCA5LjY0NTc5IDE2LjEwMjlDOS44MjA2NiAxNS45MzE5IDkuOTU5ODkgMTUuNzI3IDEwLjA1NSAxNS41MDEzQzEwLjE1MDIgMTUuMjc1MyAxMC4xOTkxIDE1LjAzMTkgMTAuMTk5NSAxNC43ODY1QzEwLjE5OTkgMTQuNTQxMSAxMC4xNTE0IDE0LjI5NzkgMTAuMDU2OSAxNC4wNzE2QzkuOTYyNDYgMTMuODQ1MyA5LjgyNDE5IDEzLjYzOTcgOS42NDk2OSAxMy40NjgxQzkuNDc1MiAxMy4yOTY1IDkuMjY4MjYgMTMuMTYxOCA5LjA0MTI5IDEzLjA3MTZDOC44MTQxNiAxMi45ODE2IDguNTcwNjMgMTIuOTM3OSA4LjMyNjQ1IDEyLjk0MzdDOC4zMzE4MiAxMi44OTE1IDguMzM1NDggMTIuODM4OSA4LjMzNTI0IDEyLjc4NjVDOC4zMzA2OCAxMi4yOTU2IDguMTMyMzkgMTEuODI2MSA3Ljc4NDQ2IDExLjQ4MThDNy40MzY1MyAxMS4xMzc0IDYuOTY3IDEwLjk0NTcgNi40Nzg3OSAxMC45NDg2SDUuMjE0MTVDNS4yMTgxOSAxMC45MjA5IDUuMjIxNDkgMTAuODkyMiA1LjIyNTg2IDEwLjg2NDZDNS40Mzk1NyA5LjUxODI2IDYuMDQ2MzIgOC4yNjUxIDYuOTY4MDUgNy4yNjQ5N0M3Ljg4OTk3IDYuMjY0OTYgOS4wODY0MyA1LjU2MTMgMTAuNDA1NiA1LjI0NDQ2WiIgZmlsbD0iY3VycmVudENvbG9yIi8+CjxwYXRoIGQ9Ik00LjA5MDYxIDEyLjIyMTNDNC4xMTUwMiAxMi44MDkyIDQuMjA0NzUgMTMuMzkyOCA0LjM1ODYyIDEzLjk2MTNDMy43MDY0MyAxNS4wNTU0IDMuNTMwODMgMTUuOTgyNyAzLjg4MDMyIDE2LjU2NjJDNC4yMjAwOSAxNy4xMzI3IDUuMDc2MyAxNy40MTMgNi4yODgzMSAxNy4zNzQzQzYuNjc1MzEgMTcuNzc3NiA3LjEwNDg2IDE4LjE0MDYgNy41NzE2OCAxOC40NTU3QzcuNTQyNTUgMTguNDYxMSA3LjUxMzIyIDE4LjQ2NzggNy40ODQwNiAxOC40NzMyQzcuMDIxNDUgMTguNTQ1NCA2LjU1NDI2IDE4LjU4MzEgNi4wODYyNyAxOC41ODU1QzUuNDcxMTYgMTguNjU3MyA0Ljg0Nzk3IDE4LjU2NTggNC4yNzkyNSAxOC4zMTk2QzMuNzEwNDQgMTguMDczNSAzLjIxNjIxIDE3LjY4MTIgMi44NDUzOCAxNy4xODI2QzIuMTQzNDggMTYuMDA2NCAyLjUxODM4IDE0LjMzMjMgMy45MDMgMTIuNDcwN0MzLjk2Mzg2IDEyLjM4NjUgNC4wMjc3NyAxMi4zMDQxIDQuMDkwNjEgMTIuMjIxM1oiIGZpbGw9ImN1cnJlbnRDb2xvciIvPgo8cGF0aCBkPSJNMTYuNTEwOSA0LjkwMjVDMTguODA3NCA0LjU1ODg0IDIwLjQ1MTcgNS4wMTY5OCAyMS4xNTQ3IDYuMTkzMDhDMjEuODU2NyA3LjM3MDU0IDIxLjQ4MTIgOS4wNDQwNCAyMC4wOTIgMTAuOTEwMUMyMC4wMjMyIDEwLjk5NjUgMTkuOTUxOCAxMS4wODA2IDE5Ljg4MTcgMTEuMTY1N0MxOS44NDczIDEwLjc5MyAxOS43ODggMTAuNDIxNSAxOS43MDAzIDEwLjA1NDVDMTkuNjU5MiA5Ljg4MjU2IDE5LjYxMDEgOS43MTMwOCAxOS41NTggOS41NDUyOUMyMC4yNzA1IDguMzk0OTIgMjAuNDc1IDcuNDIzNjIgMjAuMTEyNiA2LjgxNjcyQzE5Ljg0NDIgNi41MDg0IDE5LjUwMzIgNi4yNzI0MSAxOS4xMjA5IDYuMTMwMkMxOC43Mzg2IDUuOTg3OTQgMTguMzI2NyA1Ljk0Mzg4IDE3LjkyMzEgNi4wMDIzOEMxNy43MTk3IDYuMDAzNzEgMTcuNTE2NSA2LjAxMTkzIDE3LjMxMzkgNi4wMjgxNUMxNi45NTQgNS43MDIxNSAxNi41NjM1IDUuNDA2NjUgMTYuMTQzOSA1LjE0ODg2QzE2LjA2MTQgNS4wOTgxNCAxNS45Nzc1IDUuMDQ5OTggMTUuODkzNSA1LjAwMjQ5QzE2LjA5ODQgNC45NjUxOSAxNi4zMDQ2IDQuOTMxODkgMTYuNTEwOSA0LjkwMjVaIiBmaWxsPSJjdXJyZW50Q29sb3IiLz4KPC9zdmc+Cg=="
)

type accountDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*accountDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*accountDiscovery)(nil)
)

func NewAccountDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&accountDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *accountDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDCosmosDbAccount,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *accountDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDCosmosDbAccount,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure Cosmos DB", Other: "Azure Cosmos DBs"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure.cosmosdb.api-kind"},
				{Attribute: "azure.cosmosdb.consistency-level"},
				{Attribute: "azure.cosmosdb.public-network-access"},
				{Attribute: "azure.location"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *accountDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "azure.cosmosdb.account.name", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB account name", Other: "Cosmos DB account names"}},
		{Attribute: "azure.cosmosdb.api-kind", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB API kind", Other: "Cosmos DB API kinds"}},
		{Attribute: "azure.cosmosdb.consistency-level", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB consistency level", Other: "Cosmos DB consistency levels"}},
		{Attribute: "azure.cosmosdb.enable-multiple-write-locations", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB multi-write locations", Other: "Cosmos DB multi-write locations"}},
		{Attribute: "azure.cosmosdb.enable-automatic-failover", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB automatic failover", Other: "Cosmos DB automatic failover"}},
		{Attribute: "azure.cosmosdb.locations", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB location", Other: "Cosmos DB locations"}},
		{Attribute: "azure.cosmosdb.write-locations", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB write location", Other: "Cosmos DB write locations"}},
		{Attribute: "azure.cosmosdb.read-locations", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB read location", Other: "Cosmos DB read locations"}},
		{Attribute: "azure.cosmosdb.public-network-access", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB public network access", Other: "Cosmos DB public network access"}},
		{Attribute: "azure.cosmosdb.is-virtual-network-filter-enabled", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB VNet filter", Other: "Cosmos DB VNet filter"}},
		{Attribute: "azure.cosmosdb.disable-key-based-metadata-write-access", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB key-based metadata write disabled", Other: "Cosmos DB key-based metadata write disabled"}},
		{Attribute: "azure.cosmosdb.disable-local-auth", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB local auth disabled", Other: "Cosmos DB local auth disabled"}},
		{Attribute: "azure.cosmosdb.backup-policy.type", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB backup policy type", Other: "Cosmos DB backup policy types"}},
		{Attribute: "azure.cosmosdb.is-zone-redundant", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB zone-redundant", Other: "Cosmos DB zone-redundant"}},
		{Attribute: "azure.cosmosdb.provisioning-state", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB provisioning state", Other: "Cosmos DB provisioning states"}},
	}
}

func (d *accountDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllCosmosDbAccounts(ctx, client)
}

func getAllCosmosDbAccounts(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{&subscriptionId}
	}
	results, err := client.Resources(ctx, armresourcegraph.QueryRequest{
		Query: extutil.Ptr("Resources | where type =~ 'Microsoft.DocumentDB/databaseAccounts' | project id, name, kind, type, resourceGroup, location, tags, properties, subscriptionId"),
		Options: &armresourcegraph.QueryRequestOptions{
			ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
		},
		Subscriptions: subscriptions,
	}, nil)
	if err != nil {
		log.Error().Err(err).Msgf("failed to get Cosmos DB results")
		return nil, err
	}

	targets := make([]discovery_kit_api.Target, 0)
	rows, ok := results.Data.([]any)
	if !ok {
		return targets, nil
	}
	for _, r := range rows {
		items, ok := r.(map[string]any)
		if !ok {
			continue
		}
		targets = append(targets, toCosmosDbAccountTarget(items))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesCosmosDb), nil
}

func toCosmosDbAccountTarget(items map[string]any) discovery_kit_api.Target {
	properties := common.GetMapValue(items, "properties")
	consistency := common.GetMapValue(properties, "consistencyPolicy")
	backupPolicy := common.GetMapValue(properties, "backupPolicy")

	id, _ := items["id"].(string)
	name, _ := items["name"].(string)

	attributes := make(map[string][]string)
	attributes["azure.cosmosdb.account.name"] = []string{name}
	attributes["azure.subscription.id"] = []string{stringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{stringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{stringFromMap(items, "location")}

	if v := stringFromMap(items, "kind"); v != "" {
		attributes["azure.cosmosdb.api-kind"] = []string{v}
	}
	if v := stringFromMap(consistency, "defaultConsistencyLevel"); v != "" {
		attributes["azure.cosmosdb.consistency-level"] = []string{v}
	}
	if v, ok := properties["enableMultipleWriteLocations"].(bool); ok {
		attributes["azure.cosmosdb.enable-multiple-write-locations"] = []string{strconv.FormatBool(v)}
	}
	if v, ok := properties["enableAutomaticFailover"].(bool); ok {
		attributes["azure.cosmosdb.enable-automatic-failover"] = []string{strconv.FormatBool(v)}
	}
	if v := stringFromMap(properties, "publicNetworkAccess"); v != "" {
		attributes["azure.cosmosdb.public-network-access"] = []string{v}
	}
	if v, ok := properties["isVirtualNetworkFilterEnabled"].(bool); ok {
		attributes["azure.cosmosdb.is-virtual-network-filter-enabled"] = []string{strconv.FormatBool(v)}
	}
	if v, ok := properties["disableKeyBasedMetadataWriteAccess"].(bool); ok {
		attributes["azure.cosmosdb.disable-key-based-metadata-write-access"] = []string{strconv.FormatBool(v)}
	}
	if v, ok := properties["disableLocalAuth"].(bool); ok {
		attributes["azure.cosmosdb.disable-local-auth"] = []string{strconv.FormatBool(v)}
	}
	if v := stringFromMap(backupPolicy, "type"); v != "" {
		attributes["azure.cosmosdb.backup-policy.type"] = []string{v}
	}
	if v := stringFromMap(properties, "provisioningState"); v != "" {
		attributes["azure.cosmosdb.provisioning-state"] = []string{v}
	}

	if locs := locationNames(properties, "locations"); len(locs) > 0 {
		sort.Strings(locs)
		attributes["azure.cosmosdb.locations"] = locs
	}
	if locs := locationNames(properties, "writeLocations"); len(locs) > 0 {
		sort.Strings(locs)
		attributes["azure.cosmosdb.write-locations"] = locs
	}
	if locs := locationNames(properties, "readLocations"); len(locs) > 0 {
		sort.Strings(locs)
		attributes["azure.cosmosdb.read-locations"] = locs
	}

	if isZoneRedundant := anyZoneRedundant(properties); isZoneRedundant != nil {
		attributes["azure.cosmosdb.is-zone-redundant"] = []string{strconv.FormatBool(*isZoneRedundant)}
	}

	for k, v := range common.GetMapValue(items, "tags") {
		attributes[fmt.Sprintf("azure.cosmosdb.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDCosmosDbAccount,
		Label:      name,
		Attributes: attributes,
	}
}

func locationNames(properties map[string]any, key string) []string {
	v, ok := properties[key]
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		loc, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := loc["locationName"].(string); ok && name != "" {
			out = append(out, name)
		}
	}
	return out
}

// anyZoneRedundant returns true iff at least one of the configured locations has isZoneRedundant=true.
func anyZoneRedundant(properties map[string]any) *bool {
	v, ok := properties["locations"]
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	hasAny := false
	seen := false
	for _, e := range arr {
		loc, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if zr, ok := loc["isZoneRedundant"].(bool); ok {
			seen = true
			if zr {
				hasAny = true
				break
			}
		}
	}
	if !seen {
		return nil
	}
	return &hasAny
}

func stringFromMap(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
