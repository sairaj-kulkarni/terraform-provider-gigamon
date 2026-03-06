// Copyright (c) Gigamon, Inc.

// Implements the common constants which can be used accross the pacakge

package commonresources

import "regexp"

// Monitoring Session payload
const (
	FMUctVMirrorTrafficEnabledKey         = "uctVMirrorTrafficEnabled"
	FMUctVFilteringEnabledKey             = "uctVFilteringEnabled"
	FMSecureTunnelOnMirrorEnabledKey      = "secureTunnelOnMirrorEnabled"
	FMUctVPrecryptionEnabledKey           = "uctVPrecryptionEnabled"
	FMUctVPrecryptionFilteringEnabledKey  = "uctVPrecryptionFilteringEnabled"
	FMSecureTunnelOnPrecryptionEnabledKey = "secureTunnelOnPrecryptionEnabled"
)

// FM policy keys
const (
	FMUctVFilteringPolicyKey = "uctVFilteringPolicy"
	FMRulesKey               = "rules"
)

// FM rule field keys
const (
	FMRuleNameKey  = "ruleName"
	FMActionKey    = "action"
	FMDirectionKey = "direction"
	FMPriorityKey  = "priority"
	FMFiltersKey   = "filters"
)

// FM filter field keys
const (
	FMFilterNameKey     = "name"
	FMFilterRelationKey = "relation"
	FMFilterValueKey    = "value"
)

// Rule actions
const (
	ActionPass = "pass"
	ActionDrop = "drop"
)

// Rule directions
const (
	DirectionBidi    = "bidi"
	DirectionIngress = "ingress"
	DirectionEgress  = "egress"
)

// Filter relations
const (
	RelationEqualTo    = "EQUAL_TO"
	RelationNotEqualTo = "NOT_EQUAL_TO"
)

// Filter names
const (
	FilterPortSrc = "portSrc"
	FilterPortDst = "portDst"
	FilterIP4Src  = "ip4Src"
	FilterIP4Dst  = "ip4Dst"
	FilterIP6Src  = "ip6Src"
	FilterIP6Dst  = "ip6Dst"
	FilterProto   = "proto"
)

// Proto values used by FM
const (
	ProtoTCPNumber = "6"
	ProtoUDPNumber = "17"
)

// Policy limits
const (
	MinUctvPolicyRules       = 1
	MaxUctvPolicyRules       = 16
	MinUctvRuleFilters       = 1
	MaxUctvRuleFilters       = 5
	MinRuleNameLen           = 1
	MaxRuleNameLen           = 20
	MinRulePriority    int64 = 1
	MaxRulePriority    int64 = 8
)

// Rule name allowed chars
var RuleNameRegex = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Slices for validators
var AllowedActions = []string{
	ActionPass,
	ActionDrop,
}

var AllowedDirections = []string{
	DirectionBidi,
	DirectionIngress,
	DirectionEgress,
}

var AllowedRelations = []string{
	RelationEqualTo,
	RelationNotEqualTo,
}

var AllowedFilterNames = []string{
	FilterPortSrc,
	FilterPortDst,
	FilterIP4Src,
	FilterIP4Dst,
	FilterIP6Src,
	FilterIP6Dst,
	FilterProto,
}

var AllowedProtoValues = []string{
	ProtoTCPNumber,
	ProtoUDPNumber,
}
