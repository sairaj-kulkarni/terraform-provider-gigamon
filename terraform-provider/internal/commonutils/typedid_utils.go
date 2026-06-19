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

package commonutils

import (
	"fmt"
	"strings"
)

const TypedIDDelim = "::"

type Module string
type Type string

type TypedIDParts struct {
	Module Module
	Type   Type
	UUID   string // raw uuid string as returned by FM (can be empty)
}

// Builds TypedId in format <module>::<type>::<uuid>
func MakeTypedID(mod Module, typ Type, uuid string) (string, error) {
	m := strings.TrimSpace(string(mod))
	t := strings.TrimSpace(string(typ))
	u := strings.TrimSpace(uuid)

	if m == "" || t == "" || u == "" {
		return "", fmt.Errorf(
			"typedid: empty component (module=%q type=%q uuid=%q)",
			m, t, u,
		)
	}
	// Checks for delimiter appearing inside tokens.
	if strings.Contains(m, TypedIDDelim) ||
		strings.Contains(t, TypedIDDelim) ||
		strings.Contains(u, TypedIDDelim) {
		return "", fmt.Errorf(
			"typedid: delimiter %q not allowed in module/type/uuid (module=%q type=%q uuid=%q)",
			TypedIDDelim, m, t, u,
		)
	}

	return m + TypedIDDelim + t + TypedIDDelim + u, nil
}

// Parse TypedID into module, type, uuid
func ParseTypedID(typedID string) (TypedIDParts, error) {
	s := strings.TrimSpace(typedID)
	if s == "" {
		return TypedIDParts{}, fmt.Errorf("typedid: empty input")
	}

	parts := strings.SplitN(s, TypedIDDelim, 3)
	if len(parts) != 3 {
		return TypedIDParts{}, fmt.Errorf(
			"typedid: invalid format, expected <module>%s<type>%s<uuid>: %q",
			TypedIDDelim, TypedIDDelim, s,
		)
	}

	m := strings.TrimSpace(parts[0])
	t := strings.TrimSpace(parts[1])
	u := strings.TrimSpace(parts[2])

	if m == "" || t == "" {
		return TypedIDParts{}, fmt.Errorf("typedid: empty module/type in %q", s)
	}

	return TypedIDParts{Module: Module(m), Type: Type(t), UUID: u}, nil
}

// From typedID extract module
func ModuleFromTypedID(typedID string) (Module, error) {
	p, err := ParseTypedID(typedID)
	if err != nil {
		return "", err
	}
	return p.Module, nil
}

// From typedID extract type
func TypeFromTypedID(typedID string) (Type, error) {
	p, err := ParseTypedID(typedID)
	if err != nil {
		return "", err
	}
	return p.Type, nil
}

// From typedID extract uuid (can be empty)
func UUIDFromTypedID(typedID string) (string, error) {
	p, err := ParseTypedID(typedID)
	if err != nil {
		return "", err
	}
	return p.UUID, nil
}
