/*
The MIT License (MIT)

Copyright (c) 2016 ZiRo

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:
The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.
THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"github.com/ZiRo-/cuckgo/cuckoo"
	"github.com/gopherjs/gopherjs/js"
)

const MAXPATHLEN = 4096
const RANDOFFS = 64
const MAXLEN = 1024

type CuckooSolve struct {
	graph    *cuckoo.Cuckoo
	easiness int
	cuckoo   []int
	sols     [][]int
	nsols    int
	nthreads int
}

func NewCuckooSolve(hdr []byte, en, ms, nt int) *CuckooSolve {
	self := &CuckooSolve{
		graph:    cuckoo.NewCuckoo(hdr),
		easiness: en,
		sols:     make([][]int, 2*ms), //this isn't completley safe for high easiness
		cuckoo:   make([]int, 1+int(cuckoo.SIZE)),
		nsols:    0,
		nthreads: 1,
	}
	for i := range self.sols {
		self.sols[i] = make([]int, cuckoo.PROOFSIZE)
	}
	return self
}

func (self *CuckooSolve) path(u int, us []int) int {
	nu := 0
	for nu = 0; u != 0; u = self.cuckoo[u] {
		nu++
		if nu >= MAXPATHLEN {
			for nu != 0 && us[nu-1] != u {
				nu--
			}
			if nu < 0 {
				//fmt.Println("maximum path length exceeded")
			} else {
				//fmt.Println("illegal", (MAXPATHLEN - nu), "-cycle")
			}
			return -1
		}
		us[nu] = u
	}
	return nu
}

func (self *CuckooSolve) solution(us []int, nu int, vs []int, nv int) {
	cycle := make(map[int]*cuckoo.Edge)
	n := 0
	edg := &cuckoo.Edge{uint64(us[0]), uint64(vs[0]) - cuckoo.HALFSIZE}
	cycle[edg.HashCode()] = edg
	for nu != 0 { // u's in even position; v's in odd
		nu--
		edg := &cuckoo.Edge{uint64(us[(nu+1)&^1]), uint64(us[nu|1]) - cuckoo.HALFSIZE}
		_, has := cycle[edg.HashCode()]
		if !has {
			cycle[edg.HashCode()] = edg
		}
	}
	for nv != 0 { // u's in odd position; v's in even
		nv--
		edg := &cuckoo.Edge{uint64(vs[nv|1]), uint64(vs[(nv+1)&^1]) - cuckoo.HALFSIZE}
		_, has := cycle[edg.HashCode()]
		if !has {
			cycle[edg.HashCode()] = edg
		}
	}
	n = 0
	for nonce := 0; nonce < self.easiness; nonce++ {
		e := self.graph.Sipedge(uint64(nonce))
		has, key := contains(cycle, e)
		if has {
			self.sols[self.nsols][n] = nonce
			n++
			delete(cycle, key)
		}
	}
	if uint64(n) == cuckoo.PROOFSIZE {
		self.nsols++
	} else {
		//fmt.Println("Only recovered ", n, " nonces")
	}
}

func contains(m map[int]*cuckoo.Edge, e *cuckoo.Edge) (bool, int) {
	h := e.HashCode()
	for k, v := range m {
		if k == h && v.U == e.U && v.V == e.V { //fuck Java for making me waste time just to figure out that that's how Java does contains
			return true, k
		}
	}
	return false, 0
}

func worker(id int, solve *CuckooSolve) {
	cuck := solve.cuckoo
	us := make([]int, MAXPATHLEN)
	vs := make([]int, MAXPATHLEN)
	for nonce := id; nonce < solve.easiness; nonce += solve.nthreads {
		us[0] = (int)(solve.graph.Sipnode(uint64(nonce), 0))
		u := cuck[us[0]]
		vs[0] = (int)(cuckoo.HALFSIZE + solve.graph.Sipnode(uint64(nonce), 1))
		v := cuck[vs[0]]
		if u == vs[0] || v == us[0] {
			continue // ignore duplicate edges
		}
		nu := solve.path(u, us)
		nv := solve.path(v, vs)

		if nu == -1 || nv == -1 {
			return
		}

		if us[nu] == vs[nv] {
			min := 0
			if nu < nv {
				min = nu
			} else {
				min = nv
			}
			nu -= min
			nv -= min
			for us[nu] != vs[nv] {
				nu++
				nv++
			}
			length := nu + nv + 1
			//fmt.Println(" " , length , "-cycle found at " , id , ":" , (int)(nonce*100/solve.easiness) , "%")
			if uint64(length) == cuckoo.PROOFSIZE && solve.nsols < len(solve.sols) {
				solve.solution(us, nu, vs, nv)
			}
			continue
		}
		if nu < nv {
			for nu != 0 {
				nu--
				cuck[us[nu+1]] = us[nu]
			}
			cuck[us[0]] = vs[0]
		} else {
			for nv != 0 {
				nv--
				cuck[vs[nv+1]] = vs[nv]
			}
			cuck[vs[0]] = us[0]
		}
	}
}

func mine_cuckoo(easipct float64) string {
	maxsols := 8

	b := make([]byte, RANDOFFS)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	easy := int(easipct * float64(cuckoo.SIZE) / 100.0)
	solve := NewCuckooSolve(b, easy, maxsols, 1)

	for k := 0; k < MAXLEN-RANDOFFS; k++ {
		b = append(b, 0)
		for i := 0; i < 256; i++ {


			b[RANDOFFS+k] = byte(i)
			solve = NewCuckooSolve(b, easy, maxsols, 1)
			worker(i, solve)

			if solve.nsols > 0 {
				goto done
			}
		}
	}

done:

	if len(solve.sols) > 0 {
		c := formatProof(solve, b)
		json, _ := cuckoo.EncodeCuckooJSON(c)
		str := base64.StdEncoding.EncodeToString(json)
		return str
	} else {
		return "No Solution found."
	}
}

func formatProof(solve *CuckooSolve, b []byte) cuckoo.CuckooJSON {
	sha := sha256.Sum256(b)
	easy := uint64(solve.easiness)
	cycle := make([]uint64, len(solve.sols[0]))
	m := make(map[string]uint64)
	m["easiness"] = easy

	for i, n := range solve.sols[0] {
		cycle[i] = uint64(n)
	}

	return cuckoo.CuckooJSON{m, sha[:], cycle}
}



func main() {
	js.Global.Set("cuckoo", map[string]interface{}{
        "mine_cuckoo": mine_cuckoo,
    })
}