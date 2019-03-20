## Outstanding
### Basic feature completeness
1. World must leave things in a consistent state if there's a crash during zone migration
1. Telnet should handle migrate-in/out events (maybe audit that it handles all other events it can see)
1. mud-daemon should actually daemonize, and have a way of shutting down cleanly

### Code-readiness
1. Use a logger rather than printing "DEBUG <...>" to console (divoxx/llog ?)
1. Dev docs for concepts in package "core", in particular how changes take effect
1. Dev docs for how to add a new event type / how to add a field to an object (which touches events)

### Usability
1. Online help mechanism for telnet interface; possibly separate for gamehandler and editor
1. Online help files for all commands in telnet interfaces
1. Basic documentation for config files
1. Guard against accounts attaching to actors they don't own; allow admin accounts to do it anyway (esp. brain service)
1. Guard against "lookAtLocation" looking at locations that "current location" doesn't have an exit to
1. Manual updates to auth.db (maybe could use a better filename) should be noticed and picked up by AuthService
1. Manual updates to spawnsCfg.yaml should similarly be picked up
1. Brains need a serialized format, so they can be tweaked without recompiling
1. Brains should persist the memories into Redis or a similar store
1. BrainService should be a separate daemon, for the following reasons:
   1. So it can run under different ulimits etc. (in case it tries to eat up all the CPU or w/e)
   1. So the code can live in a different repository which is not public (so the MUD players can't see _exactly_ how the mob AI is implemented)

### Operability
1. statsd/Prometheus stats, so if things are choking we can see /why/
   1. Brendan Gregg’s USE method: utilization, saturation, and error count (rate)  (appropriate for Zone command-handling)
       1. e.g. events/second by zone
   1. Tom Wilkie’s RED method: request count (rate), error count (rate), and duration (appropriate for WSAPI responses)

### Tech debt
1. Brain uses func() callbacks made "safe" with sync.XYZ stuff; more consistent with rest of codebase to make them "safe" using callback channels.
1. No tests, almost anywhere.
   1. Existing tests are failing.

### Game mechanics
1. Allow WSAPI to look at other locations than "current location"
1. Implement a "say" command+event (shouldPersist: false)
1. Implement spells etc.

### AI
1. Advanced brain behaviors for improved believability + performance would be awesome, e.g. guards who talk to one another about criminals and their crimes, e.g. mobs that call for help
1. BrainService needs names for each AI variant, to be used when a new brain is requested by the MUD

## Recently completed
1. Add hitpoints, equipment slots, "slash"/"stab"/etc. abilities for Actors
1. LICENSE should be GPL, not AGPL
1. Allow telnet interface to "look east" "look west" etc.
1. Brain's goal executor should be more sophisticated; enough to pick up a sword off the ground if we're fighting bare handed
1. Brains should gracefully shut down when they're evicted (I have no idea what they're doing right now..)
1. BrainService needs an API for requesting a brain for a specific Actor
1. Actors should have a brain-type field (specified in spawnsCfg.yaml)
1. SpawnReap should request brains for actors with no observers (using brain-type field)
1. -cpuprofile command line flag
1. WSAPI must drop messages when the client isn't reading them (over the network) fast enough; if it doesn't, the goroutines distributing events to observers will end up blocking on the slow WSAPI observers, and depending on the set of observers present at the time of a given set of events some of them may see the events in a different order.


# GOAP comprehension...

* Neighbors for a given node are _computed_ based on met pre-conditions
   * This allows actions to be visited repeatedly if their pre-conditions are met repeatedly.
   * There needs to be an upper-bound for plan-length, to prevent infinitely looping on an action
     that has no "real" pre-conditions (e.g. "say something") and equal cost to all other actions.
```go
func neighbors(stateSoFar state, allActions []action) []action {
    var neighbors []action
    for _,action in range allActions {
    	if action.preconditionsMet(stateSoFar) {
    		neighbors = append(neighbors, action)
    	}
    } 
    return neighbors
}
```
* Post-conditions can include specific arithmetic:
   * E.g.: `sell item X`
       * Pre-condition: `count(item X) > 0`
       * Post-condition: `gold += 30`, `count(item X) -= 1`
   * Visiting the above 3x requires 3 of `item X`, and yields `stateSoFar.gold += 90; stateSoFar.itemX -= 3` 
* What is cost?
   * Could be time to execute, if we're optimizing for that
   * Could be total resource consumption, if we're optimizing for that
   * Probably wise for core algorithm to allow for different cost functions, so it can be reused for
     different activities (combat vs dealing with a shop-keeper)
* What is distance to goal / heuristic?
   * Distance to goal is, literally, a Euclidian distance in some # of dimensions
   * E.g. if we're trying to get to 100 gold, distance is `100 - stateSoFar.gold`
   * E.g. if we're trying to kill someone, distance is `stateSoFar.enemyHP`
   
Consider this scenario: a monster is standing outside a weapons shop, but has no weapons. They are
attacked by someone wearing heavy armor.
1. They could punch the attacker 100 times, which would take 50 seconds.
1. Or, they could go into the shop, buy a war-hammer, return outside, and then bludgeon the attacker
  5 times for a total of 8 seconds.

How do I ensure that the pathfinding algorithm a) finds option #2; and b) finds it in a reasonable amount
of CPU time?

Well, pathfinding without a heuristic (Dijkstra) is guaranteed to find the shortest path, by expanding
all neighbor-nodes in `lowest-total-cost-from-origin first` fashion until it encounters the goal. Since
it's basically expanding option #1 and option #2 in parallel, it may stray down option #1's path for a
few iterations, but once the next node on that path becomes as expensive as the first node on option #2's
path, it will begin exploring option #2.

**Note**: There's a hidden problem in the way I've stated the scenario: "_go into the shop ... return outside ..._";
this involves pathfinding which has a different goal (reaching a Location in the world). This is
effectively a sub-goal.

We might state the actions like this:
```yaml
startingState:
  enemyIsNearby: true
  gold: 100
  inWeaponShop: false
  enemyHP: 100
  estimatedWeaponDamage: 1
  timeToHit: 0.5 seconds
  enemyLocation: LocationA
  myLocation: LocationA

hit-enemy:
  precondition: enemyIsNearby == true
  cost: timeToHit
  postcondition: enemyHP -= estimatedWeaponDamage
obtain-better-weapon:
  precondition: gold >= 100; inWeaponShop == true
  cost: 1 second
  postcondition: estimatedWeaponDamage = 20; timeToHit = 1 second
move-to-weapon-shop:
  precondition: inWeaponShop == false
  cost: pathCostToNearestWeaponsShop() # NOTE THAT THIS IS A FUNCTION CALL
  postcondition: enemyIsNearby = false; inWeaponShop = true
move-to-enemy:
  precondition: enemyIsNearby == false
  cost: pathCostToEnemy() # NOTE THAT THIS IS A FUNCTION CALL
  postcondition: enemyisNearby = true
```    

As presented, if we're using Dijkstra's algorithm to solve for the above, it _will_ end up deciding to
purchase a weapon if the distance to the weapon shop isn't very high, which is correct.

Introducing a heuristic changes us from `lowest-total-cost-from-origin first` to
`lowest-total-cost-from-origin + estimated-cost-to-goal first`. A simplistic estimate for
`move-to-weapon-shop` might be 2x the cost of movement (leave + return) + all the same cost of hitting
the enemy 100 times bare-handed. That's fine-- the path won't be explored til we've explored hitting
the enemy bare-handed 4 times (a total of 2 seconds). But then we'll expand the first `move-to-weapon-shop`
node... and we'll find that we now have the `obtain-better-weapon` neighbor, which we might again
estimate as having node-cost + return-to-enemy-cost, and not initially explore it. But when we get to
6 bare-handed slaps, the cost of `obtain-better-weapon` + estimated cost to goal will be the same, so
we'll expand it and should find that the estimated cost of the rest of the path is much lower, and begin
exploring this path exclusively as it's muuuuuch shorter overall.

Seems like we could screw up the heuristics though... let's sketch them out:
```go
type combatState struct {
	enemyHP float64
	estimatedWeaponDamage float64
	timeToHit float64
	myLocation *Location
	enemyLocation *Location
	worldMap *WorldMap
}

type movementAction struct {
	startingLocation, endingLocation *Location
}

func (ma movementAction) estimateCostToGoal(s combatState) float64 {
	// estimate that we'll just return to our starting node;
	// this probably turns A* into Dijkstra's for guessing whether
	// movement is a good idea
	outPath := pathFind(ma.startingLocation, ma.endingLocation, s.worldMap)
	returnPath := pathFind(ma.endingLocation, ma.startingLocation, s.worldMap)
	return outPath.cost() + returnPath.cost() + ((s.enemyHP / s.estimatedWeaponDamage) * s.timeToHit)
}

type buyWeaponAction struct {
	newWeaponDamage float64
	newTimeToHit float64
}

func (bwa buyWeaponAction) estimateCostToGoal(s combatState) float64 {
	// estimate that we'll need to return to the enemy, then resume killing
	returnPath := pathFind(s.myLocation, s.enemyLocation, s.worldMap)
	return returnPath.cost() + ((s.enemyHP / bwa.newWeaponDamage) * bwa.newTimeToHit) 
}
```
