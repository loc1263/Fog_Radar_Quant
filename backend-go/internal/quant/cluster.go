package quant

import (
	"math"
	"sort"
)

// Merge es una fusión del dendrograma: ids de los dos clusters (0..n-1 son hojas, n+i la fusión i),
// la distancia y el tamaño resultante (misma convención que scipy.cluster.hierarchy.linkage).
type Merge struct {
	A, B int
	Dist float64
	Size int
}

// AverageLinkage es el clustering jerárquico aglomerativo con enlace promedio (UPGMA) sobre una matriz
// de distancias simétrica n×n (equivale a linkage(squareform(dist), method="average")).
func AverageLinkage(dist [][]float64) []Merge {
	n := len(dist)
	d := make([][]float64, n)
	for i := range d {
		d[i] = append([]float64(nil), dist[i]...)
	}
	id := make([]int, n) // id de cluster de cada fila activa
	size := make([]int, n)
	active := make([]bool, n)
	for i := range id {
		id[i], size[i], active[i] = i, 1, true
	}
	merges := make([]Merge, 0, n-1)
	for step := 0; step < n-1; step++ {
		bi, bj, best := -1, -1, math.Inf(1)
		for i := 0; i < n; i++ {
			if !active[i] {
				continue
			}
			for j := i + 1; j < n; j++ {
				if active[j] && d[i][j] < best {
					bi, bj, best = i, j, d[i][j]
				}
			}
		}
		a, b := id[bi], id[bj]
		if a > b {
			a, b = b, a
		}
		na, nb := float64(size[bi]), float64(size[bj])
		for k := 0; k < n; k++ {
			if active[k] && k != bi && k != bj {
				v := (na*d[bi][k] + nb*d[bj][k]) / (na + nb)
				d[bi][k], d[k][bi] = v, v
			}
		}
		active[bj] = false
		size[bi] += size[bj]
		id[bi] = n + step
		merges = append(merges, Merge{A: a, B: b, Dist: best, Size: size[bi]})
	}
	return merges
}

// FclusterMaxclust corta el dendrograma en a lo más k clusters (fcluster(criterion="maxclust")):
// busca el umbral más chico tal que el número de clusters sea ≤ k y devuelve la etiqueta (1..m) de
// cada hoja. Con alturas empatadas puede dar menos de k clusters, igual que scipy.
func FclusterMaxclust(merges []Merge, n, k int) []int {
	parent := make([]int, 2*n)
	for i := range parent {
		parent[i] = i
	}
	find := func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	heights := make([]float64, len(merges))
	for i, m := range merges {
		heights[i] = m.Dist
	}
	sort.Float64s(heights)

	clustersAt := func(t float64) int {
		for i := range parent {
			parent[i] = i
		}
		count := n
		for i, m := range merges {
			if m.Dist <= t {
				parent[find(m.A)] = n + i
				parent[find(m.B)] = n + i
				count--
			}
		}
		return count
	}
	t := math.Inf(-1) // sin fusiones: n clusters
	if clustersAt(t) > k {
		for _, h := range heights {
			t = h
			if clustersAt(t) <= k {
				break
			}
		}
	}
	clustersAt(t)
	labels := make([]int, n)
	next := 0
	seen := map[int]int{}
	for i := 0; i < n; i++ {
		r := find(i)
		if _, ok := seen[r]; !ok {
			next++
			seen[r] = next
		}
		labels[i] = seen[r]
	}
	return labels
}
