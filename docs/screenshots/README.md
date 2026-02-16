# Screenshots

This directory contains screenshots for the README.md file.

## Required Screenshots

### policies.png
Screenshot of the Policies page showing:
- Account selector in sidebar
- 50/50 split layout with policy list on left
- Policy details panel on right showing statements
- Create FAB button
- Search/filter controls

### bindings.png
Screenshot of the Bindings page showing:
- Account selector in sidebar
- 50/50 split layout with binding list on left
- Binding details panel on right showing:
  - Policies list with hyperlinks
  - Compiled statements section expanded
  - Aggregated permissions from multiple policies
- Policy filter dropdown

### simulator.png
Screenshot of the Simulator page showing:
- 50/50 grid layout
- Left side: User configuration form
  - User name field (filled with "alice")
  - Target account selector
  - Roles multi-select
- Right side: Compilation results
  - Permissions section showing pub/sub subjects
  - Roles & Policies section with hyperlinks
  - Raw Response section (collapsed)

## How to Capture Screenshots

1. Start the control plane: `cd ctrlp && npm start`
2. Navigate to each page
3. Ensure realistic test data is loaded (policies, bindings, etc.)
4. Capture full-width screenshots at 1920x1080 or similar resolution
5. Save as PNG files with the names above
6. Commit to this directory
