# Go MUD
This is a learning project, and possibly later a fun project.

**DISCLAIMER**: As this is a _learning_ project, I'm doing a _lot_ of code-restructuring as I go 
and consequently I am not investing much time in code quality (that is: consistently applied
patterns, unit tests, so on).

Feel free to peruse the code or even re-use it, but please understand that unless my goals demand
that e.g. I go back and fix every instance of a slightly-less-good pattern, I'm probably just
going to leave it be, and that I'm not writing tests til the code stabilizes enough + becomes
complex enough that I feel it'd _increase_ my change-velocity to add them. 

## Learning objectives
* Employ concurrency in a non-trivial system. By "non-trivial" I mean one which has enough moving
    parts that, under load, it forces me to learn:
    * When and how to correctly apply back-pressure, load-shedding, and circuit breakers to cope
      with overload.
    * How to avoid dead-lock in complex workflows; how to correctly diagnose when it happens and
      distinguish from overload and live-lock; how to decide where to make a change when a
      deadlock condition is found.
* Implement Event-Sourcing
    * To familiarize with the technique and understand it strengths/weaknesses
* Implement an event-store
    * To discover critical features and develop intuition about how they're implemented
* Employ Domain Driven Design concepts for code structure
* Employ websockets
    * To familiarize with the protocol and implementation challenges
* Employ utility-system and goal-oriented-action-planning (GOAP) AI techniques

## Code objectives
* Create a usable, event-sourced MUD
* Provide a traditional Telnet interface
* Provide an API for MUD players, so code/robots are first-class citizens (no telnet-text-parsing here!)
* Use the same API used by MUD players to write primary AI for NPCs

## Game objectives
* Create a world which is fun and interesting, and which rewards creativity and teamwork over massive
time-investment. In other words, a game which adults like me might enjoy. 

Do this by...
* Create interesting, believable, problem-solving, opportunistic AI
    * For both companions and enemies
* Create a world in which actions are individually simple, but sophisticated tactics/strategy naturally 
  arise when more than one player-character (PC) work together.
* Create challenges which reward players who either work together, or write their own AI to play
  multiple characters, in order to employ multi-PC strategies.
* Create+publish a baseline of useful AI, to encourage players to extend it for their own use.
* Make available AI-driven hirelings, with increasingly sophisticated AI (more "goal" options available
  to the employer) based on their "hourly wage", to make hard challenges accessible even to players who
  aren't strong coders and don't like depending on other players.
    * These hirelings would still need to be carefully managed, shifting the challenge from "find a
      good team" or "write a very good AI" to "carefully consider and execute a plan".

## Implemented features
### Core
* Event-sourced core; all changes in a Zone (aggregate root) are atomic,
serial, and can be replayed in a vacuum
* _Observer_-pattern for notifying non-Core code about events 
* Atomic, global (all Zones) snapshots with zero downtime
* Cross-zone migration for Actors
   * This is implemented using eventual-consistency, handled at the World level
   * World-driven eventual consistency uses intent-logging / undo semantics to guarantee consistency after crashes
### Usability
* Interactive event-stream debugger
   * Can be used to reconstruct bugs, or find state before a bug damaged data so the data can be manually restored
   * Events can be replayed one-at-a-time; a connected player can watch as they unfold normally
   * Breakpoints can be set for points-in-time, or on any events which affect certain objects
* Well structured, maintainable code
   * See the [developer documentation](doc/README.md) for more.
### Game features
* Characters can move around, pick up objects, put them inside other objects, look at things and places.
* In-game "world editor" allows the world to be modified in real-time.
* Automatic AI-process spawning for non-controlled NPCs
    * Simple "utility-system"-based AI, which chooses goals based on their relative "utility" to the NPC
* Item decay for objects left on the ground for a configurable amount of time
### Persistence
* Event persist/retrieval, filtered by Zone
* Snapshot storage
* Automatic snapshot re-play
   * E.g. if a snapshot covers events 0-1000, you'll receive the snapshot
   as a leading set of events in the same stream as events 1001+
* Configurable compression of event bodies on-disk
   * This can be toggled on/off over the course of an event stream;
   all events will be deserialized appropriately.
* Intent-logging implementation
   * Transactions are logged with undo/redo events, and later confirmed complete
   * On replay of log, unconfirmed transactions' undo/redo events are sent to a callback function for contextual
   correction-processing
### Telnet interface
* Terminal probing for height/width/type using telnet commands
* Automatic word-boundary wrapping for long display strings
* Reusable menu-system scales to terminal height
### User management
* Online account creation with default permissions
* Accounts database serialized to YAML for hand-modifying of permissions by administrator
* Passphrases hashed using bcrypt
* Passphrase hashes automatically upgraded to higher-cost target bcrypt value
  when accessed, if their current cost is lower than the target.
### Player API
* Websocket communication, so the MUD can push events as well as responses to commands
* Connections authenticated against same user database as Telnet interface
   * Uses HTTP Basic authentication before upgrading to websocket
  