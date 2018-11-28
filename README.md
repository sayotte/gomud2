# Go MUD
This is a learning project, and possibly later a fun project.

## Objectives
* Create a usable, event-sourced MUD
* Employ Domain Driven Design concepts for code structure
* Provide a traditional Telnet interface
* Provide an API for MUD players, so code/robots are first-class citizens (no telnet-text-parsing here!)
* Use the same API used by MUD players to write primary AI for NPCs

## Implemented features
### Core
* Event-sourced core; all changes in a Zone (aggregate root) are atomic,
serial, and can be replayed in a vacuum
* _Observer_-pattern for notifying non-Core code about events 
* Atomic, global (all Zones) snapshots with zero downtime
* Cross-zone migration for Actors
   * This is implemented using eventual-consistency, handled at the World level
   * World-driven eventual consistency uses intent-logging / undo semantics to guarantee consistency after crashes
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
  
