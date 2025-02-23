/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ast

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

import (
	"github.com/arana-db/parser/ast"
	"github.com/arana-db/parser/opcode"

	"github.com/pkg/errors"
)

import (
	"github.com/arana-db/arana/pkg/runtime/misc"
)

var (
	_ ExpressionAtom = (ColumnNameExpressionAtom)(nil)
	_ ExpressionAtom = (*MathExpressionAtom)(nil)
	_ ExpressionAtom = (VariableExpressionAtom)(0)
	_ ExpressionAtom = (*ConstantExpressionAtom)(nil)
	_ ExpressionAtom = (*NestedExpressionAtom)(nil)
	_ ExpressionAtom = (*FunctionCallExpressionAtom)(nil)
	_ ExpressionAtom = (*UnaryExpressionAtom)(nil)
	_ ExpressionAtom = (*SystemVariableExpressionAtom)(nil)
	_ ExpressionAtom = (*IntervalExpressionAtom)(nil)
)

var _compat80Dict = map[string]string{
	"query_cache_size": "'1048576'",
	"query_cache_type": "'OFF'",
	"tx_isolation":     "@@transaction_isolation",
	"tx_read_only":     "@@transaction_read_only",
}

type expressionAtomPhantom struct{}

type ExpressionAtom interface {
	Node
	Restorer
	phantom() expressionAtomPhantom
}

type IntervalExpressionAtom struct {
	Unit  ast.TimeUnitType
	Value PredicateNode
}

func (ie *IntervalExpressionAtom) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitAtomInterval(ie)
}

func (ie *IntervalExpressionAtom) Duration() time.Duration {
	switch ie.Unit {
	case ast.TimeUnitMicrosecond:
		return time.Microsecond
	case ast.TimeUnitSecond:
		return time.Second
	case ast.TimeUnitMinute:
		return time.Minute
	case ast.TimeUnitHour:
		return time.Hour
	case ast.TimeUnitDay:
		return time.Hour * 24
	default:
		panic(fmt.Sprintf("unsupported interval unit %s!", ie.Unit))
	}
}

func (ie *IntervalExpressionAtom) Restore(flag RestoreFlag, sb *strings.Builder, args *[]int) error {
	sb.WriteString("INTERVAL ")
	if err := ie.Value.Restore(flag, sb, args); err != nil {
		return errors.WithStack(err)
	}
	sb.WriteByte(' ')
	sb.WriteString(ie.Unit.String())

	return nil
}

func (ie *IntervalExpressionAtom) phantom() expressionAtomPhantom {
	return expressionAtomPhantom{}
}

type SystemVariableExpressionAtom struct {
	Name   string
	System bool
	Global bool
}

func (sy *SystemVariableExpressionAtom) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitAtomSystemVariable(sy)
}

func (sy *SystemVariableExpressionAtom) IsCompat80() bool {
	_, ok := _compat80Dict[sy.Name]
	return ok
}

func (sy *SystemVariableExpressionAtom) Restore(rf RestoreFlag, sb *strings.Builder, _ *[]int) error {
	if rf.Has(RestoreCompat80) {
		if compat80, ok := _compat80Dict[sy.Name]; ok {
			sb.WriteString(compat80)
			return nil
		}
	}

	sb.WriteByte('@')

	if sy.System {
		sb.WriteByte('@')
	}

	if sy.Global {
		sb.WriteString("GLOBAL.")
	}

	WriteID(sb, sy.Name)

	return nil
}

func (sy *SystemVariableExpressionAtom) phantom() expressionAtomPhantom {
	return expressionAtomPhantom{}
}

type UnaryExpressionAtom struct {
	Operator string
	Inner    Node // ExpressionAtom or *BinaryComparisonPredicateNode
}

func (u *UnaryExpressionAtom) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitAtomUnary(u)
}

func (u *UnaryExpressionAtom) IsOperatorNot() bool {
	switch u.Operator {
	case "!", "NOT":
		return true
	}
	return false
}

func (u *UnaryExpressionAtom) Restore(flag RestoreFlag, sb *strings.Builder, args *[]int) error {
	sb.WriteString(u.Operator)

	switch val := u.Inner.(type) {
	case ExpressionAtom:
		if err := val.Restore(flag, sb, args); err != nil {
			return errors.WithStack(err)
		}
	case *BinaryComparisonPredicateNode:
		if err := val.Restore(flag, sb, args); err != nil {
			return errors.WithStack(err)
		}
	default:
		panic("unreachable")
	}
	return nil
}

func (u *UnaryExpressionAtom) phantom() expressionAtomPhantom {
	return expressionAtomPhantom{}
}

type ConstantExpressionAtom struct {
	Inner interface{}
}

func (c *ConstantExpressionAtom) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitAtomConstant(c)
}

func (c *ConstantExpressionAtom) Restore(flag RestoreFlag, sb *strings.Builder, args *[]int) error {
	sb.WriteString(constant2string(c.Inner))
	return nil
}

func (c *ConstantExpressionAtom) phantom() expressionAtomPhantom {
	return expressionAtomPhantom{}
}

func constant2string(value interface{}) string {
	switch v := value.(type) {
	case Null:
		return v.String()
	case int:
		return strconv.FormatInt(int64(v), 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case string:
		var sb strings.Builder
		sb.Grow(len(v) + 16)
		sb.WriteByte('\'')
		misc.WriteEscape(&sb, v, misc.EscapeSingleQuote)
		sb.WriteByte('\'')
		return sb.String()
	case bool:
		if v {
			return "true"
		} else {
			return "false"
		}
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	default:
		panic(fmt.Sprintf("todo: render %T to string!", v))
	}
}

func (c *ConstantExpressionAtom) String() string {
	return constant2string(c.Inner)
}

func (c *ConstantExpressionAtom) IsNull() bool {
	_, ok := c.Value().(Null)
	return ok
}

func (c *ConstantExpressionAtom) Value() interface{} {
	return c.Inner
}

type ColumnNameExpressionAtom []string

func NewSingleColumnNameExpressionAtom(name string) ColumnNameExpressionAtom {
	return ColumnNameExpressionAtom{name}
}

func (c ColumnNameExpressionAtom) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitAtomColumn(c)
}

func (c ColumnNameExpressionAtom) Prefix() string {
	if len(c) > 1 {
		return c[0]
	}
	return ""
}

func (c ColumnNameExpressionAtom) Suffix() string {
	return c[len(c)-1]
}

func (c ColumnNameExpressionAtom) Restore(flag RestoreFlag, sb *strings.Builder, _ *[]int) error {
	WriteID(sb, c[0])

	for i := 1; i < len(c); i++ {
		sb.WriteByte('.')
		WriteID(sb, c[i])
	}
	return nil
}

func (c ColumnNameExpressionAtom) String() string {
	return MustRestoreToString(RestoreDefault, c)
}

func (c ColumnNameExpressionAtom) phantom() expressionAtomPhantom {
	return expressionAtomPhantom{}
}

type VariableExpressionAtom int

func (v VariableExpressionAtom) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitAtomVariable(v)
}

func (v VariableExpressionAtom) Restore(flag RestoreFlag, sb *strings.Builder, args *[]int) error {
	sb.WriteByte('?')

	if args != nil {
		*args = append(*args, v.N())
	}

	return nil
}

func (v VariableExpressionAtom) N() int {
	return int(v)
}

func (v VariableExpressionAtom) phantom() expressionAtomPhantom {
	return expressionAtomPhantom{}
}

type MathExpressionAtom struct {
	Left     ExpressionAtom
	Operator string
	Right    ExpressionAtom
}

func (m *MathExpressionAtom) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitAtomMath(m)
}

func (m *MathExpressionAtom) Restore(flag RestoreFlag, sb *strings.Builder, args *[]int) error {
	if err := m.Left.Restore(flag, sb, args); err != nil {
		return errors.WithStack(err)
	}
	switch m.Operator {
	case opcode.IntDiv.Literal():
		sb.WriteByte(' ')
		sb.WriteString(m.Operator)
		sb.WriteByte(' ')
	default:
		sb.WriteString(m.Operator)
	}

	if err := m.Right.Restore(flag, sb, args); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (m *MathExpressionAtom) phantom() expressionAtomPhantom {
	return expressionAtomPhantom{}
}

type NestedExpressionAtom struct {
	First ExpressionNode
}

func (n *NestedExpressionAtom) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitAtomNested(n)
}

func (n *NestedExpressionAtom) Restore(flag RestoreFlag, sb *strings.Builder, args *[]int) error {
	sb.WriteByte('(')
	if err := n.First.Restore(flag, sb, args); err != nil {
		return errors.WithStack(err)
	}
	sb.WriteByte(')')

	return nil
}

func (n *NestedExpressionAtom) phantom() expressionAtomPhantom {
	return expressionAtomPhantom{}
}

type FunctionCallExpressionAtom struct {
	F Node // *Function OR *AggrFunction OR *CaseWhenElseFunction OR *CastFunction
}

func (f *FunctionCallExpressionAtom) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitAtomFunction(f)
}

func (f *FunctionCallExpressionAtom) Restore(flag RestoreFlag, sb *strings.Builder, args *[]int) error {
	var err error
	switch v := f.F.(type) {
	case *Function:
		err = v.Restore(flag, sb, args)
	case *AggrFunction:
		err = v.Restore(flag, sb, args)
	case *CaseWhenElseFunction:
		err = v.Restore(flag, sb, args)
	case *CastFunction:
		err = v.Restore(flag, sb, args)
	default:
		panic("unreachable")
	}

	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (f *FunctionCallExpressionAtom) phantom() expressionAtomPhantom {
	return expressionAtomPhantom{}
}
