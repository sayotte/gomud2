== Outstanding
=== Basic feature completeness
1. World must leave things in a consistent state if there's a crash during zone migration
1. Telnet should handle migrate-in/out events (maybe audit that it handles all other events it can see)
1. Brains should gracefully shut down when they're evicted (I have no idea what they're doing right now..)
1. mud-daemon should actually daemonize, and have a way of shutting down cleanly

=== Code-readiness
1. LICENSE should be GPL, not AGPL
1. Use a logger rather than printing "DEBUG <...>" to console (divoxx/llog ?)
1. Dev docs for concepts in package "core", in particular how changes take effect
1. Dev docs for how to add a new event type / how to add a field to an object (which touches events)

=== Usability
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

=== Operability
1. statsd/Prometheus stats, so if things are choking we can see /why/
   1. Brendan Gregg’s USE method: utilization, saturation, and error count (rate)  (appropriate for Zone command-handling)
       1. e.g. events/second by zone
   1. Tom Wilkie’s RED method: request count (rate), error count (rate), and duration (appropriate for WSAPI responses)

=== Tech debt
1. Brain uses func() callbacks made "safe" with sync.XYZ stuff; more consistent with rest of codebase to make them "safe" using callback channels.
1. No tests, almost anywhere.
   1. Existing tests are failing.

=== Game mechanics
1. Allow telnet interface to "look east" "look west" etc.
1. Allow WSAPI to look at other locations than "current location"
1. Implement a "say" command+event (shouldPersist: false)
1. Add hitpoints, equipment slots, "slash"/"stab"/etc. abilities for Actors
1. Implement spells etc.

=== AI
1. Brain's goal executor should be more sophisticated; enough to pick up a sword off the ground if we're fighting bare handed
1. Advanced brain behaviors for improved believability + performance would be awesome, e.g. guards who talk to one another about criminals and their crimes, e.g. mobs that call for help


== Recently completed
1. BrainService needs names for each AI variant, to be used when a new brain is requested by the MUD
1. BrainService needs an API for requesting a brain for a specific Actor
1. Actors should have a brain-type field (specified in spawnsCfg.yaml)
1. SpawnReap should request brains for actors with no observers (using brain-type field)
1. -cpuprofile command line flag
1. WSAPI must drop messages when the client isn't reading them (over the network) fast enough; if it doesn't, the goroutines distributing events to observers will end up blocking on the slow WSAPI observers, and depending on the set of observers present at the time of a given set of events some of them may see the events in a different order.
