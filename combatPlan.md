== ATTRIBUTES
=== Base attributes
strength - how hard do you hit / much can you carry
fitness - how long can your body keep going
will - how hard do you try / how long can your mind keep going
faith(deity) - how deep is your connection to your patron deity?

each base attribute is 1-100, max 300 total base-attribute points
base attributes are raised through using skills which favor them
    translation: if it receives a bonus from the attribute, it may raise the attribute


=== Derived / current attributes
str->phys: scales melee damage, physical action speed (absolute lowered delays)
fitness->stam: scales all action speed (% lowered delays)
will->focus: scales sorcery effectiveness, physical effectiveness (to a lesser degree)
faith->zeal: scales prayer effectiveness, resistance to enemy prayers

== SKILLS
slashing - cutting with the edge of a weapon
    damages: phys:stam:focus @ 3:1:1
        phys+50%, focus+15%
    training: repetition (repetition is # of damage points dealt, on scale of 0-<configurable number>)
stabbing - puncturing with the tip of a weapon
    damages: phys:stam:focus @ 2:2:1
        phys+50%, focus+15%
    training: repetition
bashing - crushing with the blunt part of a weapon
    damages: phys:stam:focus @ 2:1:2
        phys+50%, focus+15%
    training: repetition

dodging - evading an attack so it doesn't touch you at all
    bonuses: stam+35%, focus+35%
    training: repetition, reading scrolls for techniques (e.g. perhaps 5 techniques total)
    invocation:
        automatic
        each technique-evaluation gets a base chance:
            clamp(-50, 50, defender_skill-attacker_skill)+50 * 20%, then convert to %
            i.e. at 50 attacker skill-advantage, the % chance for each dodge is 0
            i.e. at 0 attacker skill-advantage, the % chance for each dodge is 10
            i.e. at -50 attacker skill-advantage, the % chance for each dodge is 20
        each technique gets the stam/focus bonus
            at 100stam/100focus, with a -50 attacker advantage, each technique would have a 34% chance
        techniques allow additional chances for success (up to total # of techniques known)
            additional techniques are only invoked at their "skill-requirement" level
            e.g. if the 5th technique requires 80 skill, at 79 skill it won't be invoked

            example:
                atk100skill vs def100skill, 100 stam/focus, 5 techniques:  17% x 5 (60.60% total)
                atk100skill vs def60skill, 50 stam/focus, 4 techniques:   2.7% x 4 (10.37% total)
                atk60skill vs def100skill, 50 stam/focus, 5 techniques: 24.30% x 5 (75.14% total)
                atk0skill vs def100skill, 100 stam/focus, 5 techniques:    27% x 5 (79.27% total)

deflecting - using a weapon, armor, or hand to push an attack aside
    bonuses: phys+20%, stam+30%, focus+20%
    training: repetition, reading scrolls for techniques
blocking - using a shield, armor, or hand to absorb an attack
    bonuses: phys+35%, focus+35%
    training: repetition, reading scrolls for techniques

sorcery - behaving as a catalyst/conduit to selectively increase entropy, e.g. by aligning charged
  particles to channel a lightning bolt; consumes work-resources, but is not aligned with any deity
    bonuses: focus+50%
    training:
        repetition
        reading scrolls for reactions (i.e. spells, explicitly invoked)
    invocation:
        explicit, e.g. "invoke lightning mongbat"
        reactions have an innate skill-requirement; % chance of success is:
            -20 skill: 0%
            -10 skill: 50%
            <0> skill: 75%
            +10 skill: 100%
        reactions have an innate effect magnitude, modified by focus bonus
        reactions have a focus cost, e.g. 10 focus for a heal, 50 focus for

mysticism - behaving as a servant of a particular deity, who can be called upon to aid you in your
  activities through prayer; prayers are more or less effective based on your zeal, and also on
  your dedication to that deity (length of association, recent acceptable sacrifices to them) and
  the deity themselves' relative strength (sacrifices make a deity stronger, answered prayers weaker)
    bonuses:
      1:1 scale with "skill"+zeal, i.e. 0 == 0 effect and 0 resistance, 100 == max effect and immunity
    training:
        all prayers are granted immediately upon dedication to a deity
        "skill" scales directly with recent sacrifices, which decay slowly (say -2.5 per day)
        "skill" is capped by deity strength
    invocation:
        explicit, e.g. "pray punish senator" or "pray heal"
        prayers are completely unavailable below certain "skill" levels, e.g. "slay" may not work below 99
        prayer efficacy scales with "skill"+zeal


scribing - copying scrolls (techniques, reactions)
    bonuses: focus+