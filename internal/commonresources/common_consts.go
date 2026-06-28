//  Copyright (c) 2017-2026 Gigamon, Inc. All rights reserved.
//
//  Author: Gigamon Terraform Team (gigamon-terraform-team@gigamon.com)
//
//  This program is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, version 3 of the License.
//
//  This program is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with this program. If not, see <https://www.gnu.org/licenses/>

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

// Proto values used in Terraform HCL
const (
	ProtoTCP = "TCP"
	ProtoUDP = "UDP"
)

// Proto numeric values used by FM API
const (
	FMProtoTCPNumber = "6"
	FMProtoUDPNumber = "17"
)

// ProtoToFM maps user-facing proto names to FM numeric values.
var ProtoToFM = map[string]string{
	ProtoTCP: FMProtoTCPNumber,
	ProtoUDP: FMProtoUDPNumber,
}

// ProtoFromFM maps FM numeric proto values to user-facing names.
var ProtoFromFM = map[string]string{
	FMProtoTCPNumber: ProtoTCP,
	FMProtoUDPNumber: ProtoUDP,
}

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
	ProtoTCP,
	ProtoUDP,
}
