## Phase-3 required changes

This document describes which scripts and how have to be run in order to 
update indexer state for phase-3.

Scripts to run:
1. fill_max_finality_providers

### fill_max_finality_providers

**What does this script do?**

It checks that last version of staking params has correct MaxFinalityProviders
value in params. If there are discrepancies it updates only this property
in last version of staking params.

**How to run**
1. Go to directory where indexer binary resides
2. Run `./binary fill-max-fp --dry-run` (it will run the script without modifying anything)
3. If there are no error messages and no concerning log messages proceed to next step
4. Run `./binary fill-max-fp`