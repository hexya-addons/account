package account

import (
	"testing"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	. "github.com/smartystreets/goconvey/convey"
)

type TestFiscalPositionStruct struct {
	Be      m.CountrySet
	Fr      m.CountrySet
	Mx      m.CountrySet
	Eu      m.CountryGroupSet
	StateFr m.CountryStateSet
	Jc      m.PartnerSet
	Ben     m.PartnerSet
	George  m.PartnerSet
	Alberto m.PartnerSet
	BeNat   m.AccountFiscalPositionSet
	FrB2C   m.AccountFiscalPositionSet
	FrB2B   m.AccountFiscalPositionSet
}

func initTestFiscalPositionStruct(env models.Environment) TestFiscalPositionStruct {
	var out TestFiscalPositionStruct

	// reset any existing FP
	h.AccountFiscalPosition().NewSet(env).SearchAll().Write(
		h.AccountFiscalPosition().NewData().SetAutoApply(false))

	out.Be = h.Country().NewSet(env).GetRecord("base_be")
	out.Fr = h.Country().NewSet(env).GetRecord("base_fr")
	out.Mx = h.Country().NewSet(env).GetRecord("base_mx")
	out.Eu = h.CountryGroup().NewSet(env).GetRecord("base_europe")

	out.StateFr = h.CountryState().Create(env, h.CountryState().NewData().
		SetName("State").
		SetCode("ST").
		SetCountry(out.Fr))

	out.Jc = h.Partner().Create(env, h.Partner().NewData().
		SetName("JCVD").
		SetVAT("BE0477472701").
		//SetNotifyEmail("none").
		SetCountry(out.Be))
	out.Ben = h.Partner().Create(env, h.Partner().NewData().
		SetName("BP").
		//SetNotifyEmail("none").
		SetCountry(out.Be))
	out.George = h.Partner().Create(env, h.Partner().NewData().
		SetName("George").
		SetVAT("FR0477472701").
		//SetNotifyEmail("none").
		SetCountry(out.Fr))
	out.Alberto = h.Partner().Create(env, h.Partner().NewData().
		SetName("Alberto").
		SetVAT("MX0477472701").
		//SetNotifyEmail("none").
		SetCountry(out.Mx))

	out.BeNat = h.AccountFiscalPosition().Create(env,
		h.AccountFiscalPosition().NewData().
			SetName("BE-NAT").
			SetAutoApply(true).
			SetCountry(out.Be).
			SetVatRequired(false).
			SetSequence(10))
	out.FrB2C = h.AccountFiscalPosition().Create(env,
		h.AccountFiscalPosition().NewData().
			SetName("EU-VAT-FR-B2C").
			SetAutoApply(true).
			SetCountry(out.Fr).
			SetVatRequired(false).
			SetSequence(40))
	out.FrB2B = h.AccountFiscalPosition().Create(env,
		h.AccountFiscalPosition().NewData().
			SetName("EU-VAT-FR-B2B").
			SetAutoApply(true).
			SetCountry(out.Fr).
			SetVatRequired(true).
			SetSequence(50))

	return out
}

func Test10FpCountry(t *testing.T) {
	Convey("Test 10 fp country", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {

			assertFP := func(partner m.PartnerSet, expectedPos m.AccountFiscalPositionSet) {
				fiscalPos := h.AccountFiscalPosition().NewSet(env).GetFiscalPosition(partner, h.Partner().NewSet(env))
				So(fiscalPos.Equals(expectedPos), ShouldBeTrue)
			}

			self := initTestFiscalPositionStruct(env)

			// B2B has precedence over B2C for same country even when sequence gives lower precedence
			So(self.FrB2B.Sequence(), ShouldBeGreaterThan, self.FrB2C.Sequence())
			assertFP(self.George, self.FrB2B)
			self.FrB2B.SetAutoApply(false)
			assertFP(self.George, self.FrB2C)
			self.FrB2B.SetAutoApply(true)

			// Create positions matching on Country Group and on NO country at all
			euIntraB2B := h.AccountFiscalPosition().Create(env,
				h.AccountFiscalPosition().NewData().
					SetName("EU-INTRA B2B").
					SetAutoApply(true).
					SetCountryGroup(self.Eu).
					SetVatRequired(true).
					SetSequence(20))
			world := h.AccountFiscalPosition().Create(env,
				h.AccountFiscalPosition().NewData().
					SetName("WORLD-EXTRA").
					SetAutoApply(true).
					SetVatRequired(false).
					SetSequence(40))

			//Country match has higher precedence than group match or sequence
			So(self.FrB2B.Sequence(), ShouldBeGreaterThan, euIntraB2B.Sequence())
			assertFP(self.George, self.FrB2B)

			// B2B has precedence regardless of country or group match
			So(euIntraB2B.Sequence(), ShouldBeGreaterThan, self.BeNat.Sequence())
			assertFP(self.Jc, euIntraB2B)

			// Lower sequence = higher precedence if country/group and VAT matches
			So(self.Ben.VAT(), ShouldEqual, "") //No VAT set
			assertFP(self.Ben, self.BeNat)

			// Remove BE from EU group, now BE-NAT should be the fallback match before the wildcard WORLD
			self.Be.SetCountryGroups(self.Be.CountryGroups().Subtract(self.Eu))
			So(self.Jc.VAT(), ShouldNotEqual, "")
			assertFP(self.Jc, self.BeNat)

			// No country = wildcard match only if nothing else matches
			So(self.Alberto.VAT(), ShouldNotEqual, "") //with VAt
			assertFP(self.Alberto, world)
			self.Alberto.SetVAT("") //or without
			assertFP(self.Alberto, world)

			// Zip range
			frB2BZip100 := self.FrB2B.Copy(h.AccountFiscalPosition().NewData().SetZipFrom("0").SetZipTo("5000").SetSequence(60))
			self.George.SetZip("6000")
			assertFP(self.George, self.FrB2B)
			self.George.SetZip("3000")
			assertFP(self.George, frB2BZip100)

			// States
			frB2BState := self.FrB2B.Copy(h.AccountFiscalPosition().NewData().SetStates(self.StateFr).SetSequence(70))
			self.George.SetState(self.StateFr)
			assertFP(self.George, frB2BZip100)
			self.George.SetZip("0")
			assertFP(self.George, frB2BState)

			// Dedicated position has max precedence
			self.George.SetPropertyAccountPosition(self.BeNat)
			assertFP(self.George, self.BeNat)

		}), ShouldBeNil)
	})
}
