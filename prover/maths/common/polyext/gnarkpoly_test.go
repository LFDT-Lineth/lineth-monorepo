package polyext

// TODO FIX-ME
// func TestGnarkEval(t *testing.T) {

// 	t.Run("normal-poly", func(t *testing.T) {

// 		def := func(api frontend.API) error {
// 			var (
// 				pol = vectorext.IntoGnarkAssignment(vectorext.ForTestFromQuads(1, 2, 3, 4, -1, -2))
// 				x   = koalagnark.Ext{
// 					B0: gnarkfext.E2{
// 						A0: 2,
// 						A1: 1,
// 					},
// 					B1: gnarkfext.E2{
// 						A0: 0,
// 						A1: 0,
// 					},
// 				}
// 				expected = koalagnark.Ext{
// 					B0: gnarkfext.E2{
// 						A0: -5*fext.RootPowers[1] + 3,
// 						A1: -2*fext.RootPowers[1] + 1,
// 					},
// 					B1: gnarkfext.E2{
// 						A0: 0,
// 						A1: 0,
// 					},
// 				}
// 				res = EvaluateUnivariateGnark(api, pol, x)
// 			)
// 			expected.AssertIsEqual(api, res)
// 			return nil
// 		}

// 		gnarkutil.AssertCircuitSolved(t, def)
// 	})

// 	t.Run("empty-poly", func(t *testing.T) {
// 		def := func(api frontend.API) error {
// 			var (
// 				pol = vectorext.IntoGnarkAssignment([]fext.Element{})
// 				x   = koalagnark.Ext{B0: gnarkfext.E2{
// 					A0: 2,
// 					A1: 3,
// 				},
// 					B1: gnarkfext.E2{
// 						A0: 0,
// 						A1: 0,
// 					},
// 				}
// 				expected koalagnark.Ext
// 				res      = EvaluateUnivariateGnark(api, pol, x)
// 			)
// 			expected.AssertIsEqual(api, res)
// 			return nil
// 		}
// 		gnarkutil.AssertCircuitSolved(t, def)
// 	})

// }

// func TestGnarkEvalAnyDomain(t *testing.T) {

// 	t.Run("single-variable", func(t *testing.T) {

// 		def := func(api frontend.API) error {
// 			var (
// 				domain = vectorext.IntoGnarkAssignment(vectorext.ForTestFromQuads(0, 0))
// 				x      = koalagnark.Ext{B0: gnarkfext.E2{
// 					A0: 42,
// 					A1: 0,
// 				},
// 					B1: gnarkfext.E2{
// 						A0: 0,
// 						A1: 0,
// 					},
// 				}
// 				expected = vectorext.IntoGnarkAssignment(vectorext.ForTestFromQuads(1, 0))
// 				res      = EvaluateLagrangeAnyDomainGnark(api, domain, x)
// 			)

// 			require.Len(t, res, len(expected))
// 			for i := range expected {
// 				expected[i].AssertIsEqual(api, res[i])
// 			}

// 			return nil
// 		}

// 		gnarkutil.AssertCircuitSolved(t, def)
// 	})

// 	t.Run("multiple-variable", func(t *testing.T) {

// 		def := func(api frontend.API) error {
// 			var (
// 				domain = vectorext.IntoGnarkAssignment(vectorext.ForTestFromQuads(0, 0, 1, 0))
// 				x      = koalagnark.Ext{B0: gnarkfext.E2{
// 					A0: 42,
// 					A1: 0,
// 				},
// 					B1: gnarkfext.E2{
// 						A0: 0,
// 						A1: 0,
// 					},
// 				}
// 				expected = vectorext.IntoGnarkAssignment(vectorext.ForTestFromQuads(-41, 0, 42, 0))
// 				res      = EvaluateLagrangeAnyDomainGnark(api, domain, x)
// 			)

// 			require.Len(t, res, len(expected))
// 			for i := range expected {
// 				expected[i].AssertIsEqual(api, res[i])
// 			}

// 			return nil
// 		}

// 		gnarkutil.AssertCircuitSolved(t, def)
// 	})

// }
