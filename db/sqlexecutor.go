package db

import (
	"context"
	"github.com/glennliao/apijson-go/config"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"regexp"
	"strings"
)

type SqlExecutor struct {
	ctx   context.Context
	Table string

	Where [][]any //保存where条件 [ ["user_id",">", 123], ["user_id","<=",345] ]

	Columns []string
	Order   string
	Group   string

	WithEmptyResult bool // 是否最终为空结果, 用于node中中断数据获取
}

func NewSqlExecutor(ctx context.Context, tableName string, accessVerify bool) (*SqlExecutor, error) {

	return &SqlExecutor{
		ctx:   ctx,
		Table: tableName,
		Where: [][]any{},
		Order: "",
		Group: "",
	}, nil
}

// ParseCondition 解析查询条件
func (e *SqlExecutor) ParseCondition(conditions g.MapStrAny) error {

	for key, condition := range conditions {
		switch {
		case strings.HasSuffix(key, "{}"):
			e.parseMultiCondition(key[0:len(key)-2], condition)

		case strings.HasSuffix(key, "$"):
			e.Where = append(e.Where, []any{key[0 : len(key)-1], "LIKE", gconv.String(condition)})

		case strings.HasSuffix(key, "~"):
			e.Where = append(e.Where, []any{key[0 : len(key)-1], "REGEXP", gconv.String(condition)})

		default:
			e.Where = append(e.Where, []any{key, "=", condition})
		}
	}
	return nil
}

// ParseCondition 解析批量查询条件
func (e *SqlExecutor) parseMultiCondition(k string, condition any) {

	var conditions [][]string
	var value any = condition

	if _str, ok := condition.(string); ok {
		for _, s := range strings.Split(_str, ",") {
			var item []string
			ops := []string{"<=", "<", ">=", ">"}
			isEq := true
			for _, op := range ops {
				if strings.HasPrefix(s, op) {
					item = append(item, op, s[len(op):])
					isEq = false
					break
				}
			}
			if isEq {
				item = append(item, " = ", s)
			}
			conditions = append(conditions, item)
		}
		value = conditions
	}

	getK := func(k string) string {
		return k[0 : len(k)-1]
	}

	switch k[len(k)-1] {
	case '&', '|', '!':
		e.Where = append(e.Where, []any{getK(k), k[len(k)-1], value})
	default:
		e.Where = append(e.Where, []any{k, "in", value})

	}

}

var exp = regexp.MustCompile(`^[\s\w][\w()]+`) // 匹配 field, COUNT(field)

// ParseCtrl 解析 @column,@group等控制类
func (e *SqlExecutor) ParseCtrl(ctrl g.Map) error {

	fieldStyle := config.GetDbFieldStyle()

	for k, v := range ctrl {
		// https://github.com/Tencent/APIJSON/blob/master/Document.md
		// 应该用分号 ; 隔开 SQL 函数，改为 "@column":"store_id;sum(amt):totAmt"）
		fieldStr := strings.ReplaceAll(gconv.String(v), ";", ",")

		fieldList := strings.Split(fieldStr, ",")

		for i, item := range fieldList {
			fieldList[i] = exp.ReplaceAllStringFunc(item, func(field string) string {
				return fieldStyle(e.ctx, e.Table, field)
			}) // 将请求字段转化为数据库字段风格
		}

		switch k {

		case "@column":
			e.Columns = fieldList

		case "@order":
			fieldStr = strings.Join(fieldList, ",")
			order := strings.ReplaceAll(fieldStr, "-", " DESC")
			order = strings.ReplaceAll(order, "+", " ")
			e.Order = order

		case "@group":
			fieldStr = strings.Join(fieldList, ",")
			e.Group = fieldStr
		}
	}

	return nil
}

func (e *SqlExecutor) build() *gdb.Model {

	m := g.DB().Model(e.Table)

	if e.Order != "" {
		m = m.Order(e.Order)
	}

	whereBuild := m.Builder()

	fieldStyle := config.GetDbFieldStyle()

	for _, whereItem := range e.Where {
		key := fieldStyle(e.ctx, e.Table, whereItem[0].(string))
		op := whereItem[1]
		value := whereItem[2]

		if conditions, ok := value.([][]string); ok { // multiCondition

			switch op {
			case '&':
				b := m.Builder()
				for _, c := range conditions {
					b = b.Where(key+" "+c[0], c[1])
				}
				whereBuild = whereBuild.Where(b)

			case '|':
				b := m.Builder()
				for _, c := range conditions {
					b = b.WhereOr(key+" "+c[0], c[1])
				}
				whereBuild = whereBuild.Where(b)

			case '!':
				whereBuild = whereBuild.WhereNotIn(key, conditions)

			default:
				whereBuild = whereBuild.WhereIn(key, conditions)
			}
		} else {

			switch op {
			case "LIKE":
				whereBuild = whereBuild.WhereLike(key, value.(string))
			case "REGEXP":
				whereBuild = whereBuild.Where(key+" REGEXP ", value.(string))
			case "=":
				whereBuild = whereBuild.Where(key, value)
			}

		}
	}

	m = m.Where(whereBuild)

	if e.Group != "" {
		m = m.Group(e.Group)
	}

	return m
}

func (e *SqlExecutor) List(page int, count int, needTotal bool) (list []g.Map, total int, err error) {

	if e.WithEmptyResult {
		return nil, 0, err
	}

	m := e.build()

	if needTotal {
		total, err = m.Count()
		if err != nil || total == 0 {
			return nil, 0, err
		}
	}

	m = m.Fields(e.column())

	m = m.Page(page, count)
	all, err := m.All()

	if err != nil {
		return nil, 0, err
	}

	return all.List(), total, nil
}

func (e *SqlExecutor) One() (g.Map, error) {
	if e.WithEmptyResult {
		return nil, nil
	}

	m := e.build()

	m = m.Fields(e.column())

	one, err := m.One()

	return one.Map(), err
}

func (e *SqlExecutor) column() []string {

	columns := e.Columns

	if columns == nil {
		var _columns []string
		for _, column := range tableMap[e.Table].Columns {
			_columns = append(_columns, column.Name)
		}
		columns = _columns
	}

	var fields = make([]string, 0, len(columns))

	fieldStyle := config.GetJsonFieldStyle()

	for _, column := range columns {
		column = strings.ReplaceAll(column, ":", " AS ")
		if !strings.Contains(column, " AS ") {
			field := fieldStyle(e.ctx, e.Table, column)
			if field != column {
				column = column + " AS " + field
			}
		}
		fields = append(fields, column)
	}

	return fields
}
