There's a mercenary named Bob.
Bob can be hired as a guard / attacker.
Bob automatically defends his employer with lethal force.
Bob can be ordered to stop attacking all current targets.
Bob can be ordered to treat someone else as a friend of the employer.
  Friends are defended with lower priority than the employer.
  Friends are given some leeway before they are attacked.
  Friends may not issue orders like an employer does.
Bob can be ordered to attack someone else, or treat them as an enemy.
Bob can be ordered to guard one person with priority (higher priority than employer).
Bob can be ordered to stay in one location.
Bob can be asked what he is doing right now:
  I'm guarding this location.
  I'm guarding <some person>.
  I've been ordered to attack <some person> but I don't see them, so I'm following my employer.
  I'm fighting <some person> right now.
  I've been ordered to stand down, so I'm just following my employer for now.

There's a trainer named Harry.
Harry can be hired to spar with you, to train your combat skills, for a limited time.
Harry doesn't automatically retaliate when his employer attacks him.
Harry can be ordered to attack his employer, and will do so with intent to do steady but low damage.
    ... Planner should be given a goal of reducing employer to 10% HP.
    ... Planner should..... <think more on how to get it to prefer low damage>
Harry can be ordered to stop attacking his employer.
Harry warns his employer when he's hurt and needs a break.
Harry will fire his employer if they continue hurting him after he says he's hurt.
Harry will defend himself against non-employer attackers, with deadly weapons and intent to kill.

There's a guard named Joe, and another guard named Mike.
All guards will defend all other guards, with priority.
All guards have a list of known-citizens they will treat as friends.
Guards observe crimes and deliver corporal or capital punishments.
If a guard fails to complete punishment, he will remember the crime.
Guards can ask other guards about recent crimes they know about.
  Do you know about Bob attacking Harry?
  At 4pm? Yes.
  What about Harry attacking Bob?
  I don't know about that, tell me more.
  About 4:01pm, Harry attacked Bob viciously. Justice has not yet been served.
  Ok, I'll keep an eye out for Harry.
  What about Jim hurting Jerry?
  At 5pm? Yes.
  No, at 6pm. Jim hurt Jerry but not egregiously. He’s been punished, but we should watch for further infractions.


Dialogue should be rich (lots of information can be exchanged) and expository (a human should readily understand it).
Dialogue can be formulaic, even if it's rich. Repetition hurts believability, but synonymous forms are an option.
Inputs from humans are the hard part, since they aren't necessarily formulaic.
Humans may expect a simplified language anyway, i.e. we can coerce them into being formulaic without a huge breach in believability.
A mob can't know the difference between a human and an NPC, so we have to allow for all human input formulae.
This might be fine. Here are some example inputs that might be reasonable:
  Bob, kill Jamie.
  Kill Jamie, Bob.
  Attack Jamie now Bob.
  Attack Jamie Bob, now!
  Cease attacking, Bob.
  Cease attacking Bob, Jamie.
Here is an input that's perhaps unreasonable:
  Attack that scoundrel Jamie, even though you would prefer to stop attacking Jamie, Bob.
We need a framework that gives us the former without investing unnecessarily in achieving the latter.
We could perhaps use the Prose library to do segmentation and NER, but it's quite slow... 20-50ms for simple sentences.
  Would Prose be faster if retrained on a simpler model?
