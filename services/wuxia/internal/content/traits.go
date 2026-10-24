package content

// Add combines equipment and profession bonuses without dropping less common traits.
func (t Traits) Add(b Traits) Traits {
	t.Critical += b.Critical
	t.Dodge += b.Dodge
	t.Guard += b.Guard
	t.Pierce += b.Pierce
	t.CriticalDamage += b.CriticalDamage
	t.Combo += b.Combo
	t.Poison += b.Poison
	t.LifeSteal += b.LifeSteal
	return t
}
