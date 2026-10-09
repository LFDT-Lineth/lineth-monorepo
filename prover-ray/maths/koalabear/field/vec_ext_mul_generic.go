//go:build purego || !amd64

package field

// vecMulExt sets res[i] = a[i]·b[i]; res may alias a or b.
func vecMulExt(res, a, b []Ext) {
	for i := range res {
		res[i].Mul(&a[i], &b[i])
	}
}
