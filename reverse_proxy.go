package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/influxdata/influxql"
)

var (
	addr = flag.String("listen.addr", "127.0.0.1:9094", "listen address")
	dest = flag.String("influx.path", "http://influx-node:8086", "influx address")
)

func main() {
	flag.Parse()

	u, err := url.Parse(*dest)
	if err != nil {
		log.Fatalln(err)
	}

	director := func(req *http.Request) {
		req.URL.Scheme = u.Scheme
		req.URL.Host = u.Host
		req.Host = u.Host

		path := strings.SplitN(req.URL.Path, "/", 3)
		if len(path) != 3 || path[0] != "" || path[2] != "query" {
			log.Printf("invalid path: %v", req.URL)
			req.URL = nil
			return
		}

		client := path[1]
		req.URL.Path = "/query"

		uquery := req.URL.Query()
		query := uquery.Get("q")

		q, err := influxql.ParseQuery(query)
		if err != nil {
			log.Printf("invalid query (%s): %v", query, err)
			req.URL = nil
			return
		}

		for i, st := range q.Statements {
			switch st := st.(type) {
			case *influxql.SelectStatement:
				st, err := rewriteSelect(client, st)
				if err != nil {
					log.Printf("failed rewrite (%s): %v", query, err)
					req.URL = nil
					return
				}

				q.Statements[i] = st
			case *influxql.ShowTagValuesStatement:
				q.Statements[i].(*influxql.ShowTagValuesStatement).Condition = rewriteCond(client, st.Condition)
			case *influxql.ShowTagKeysStatement,
				*influxql.ShowFieldKeysStatement,
				*influxql.ShowMeasurementsStatement,
				*influxql.ShowDatabasesStatement,
				*influxql.ShowRetentionPoliciesStatement:
				// Allowed
			default:
				// Ban by default
				spew.Dump("unknown statement", st)
				req.URL = nil
				return
			}
		}

		uquery.Set("q", q.Statements.String())
		req.URL.RawQuery = uquery.Encode()
	}

	proxy := &httputil.ReverseProxy{Director: director}
	log.Fatalln(http.ListenAndServe(*addr, proxy))
}

func rewriteCond(client string, expr influxql.Expr) influxql.Expr {
	if expr == nil {
		return &influxql.BinaryExpr{
			Op:  influxql.EQ,
			LHS: &influxql.VarRef{Val: "client"},
			RHS: &influxql.StringLiteral{Val: client},
		}
	}

	return &influxql.BinaryExpr{
		Op: influxql.AND,
		LHS: &influxql.BinaryExpr{
			Op:  influxql.EQ,
			LHS: &influxql.VarRef{Val: "client"},
			RHS: &influxql.StringLiteral{Val: client},
		},
		RHS: expr,
	}
}

func rewriteSelect(client string, st *influxql.SelectStatement) (*influxql.SelectStatement, error) {
	for i, src := range st.Sources {
		switch src := src.(type) {
		case *influxql.Measurement:
		case *influxql.SubQuery:
			s, err := rewriteSelect(client, src.Statement)
			if err != nil {
				return nil, err
			}

			st.Sources[i].(*influxql.SubQuery).Statement = s
		default:
			return nil, fmt.Errorf("unknown source: %T", src)
		}
	}

	st.Condition = rewriteCond(client, st.Condition)

	return st, nil
}
