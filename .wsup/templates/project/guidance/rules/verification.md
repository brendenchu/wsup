# Verification

Choose checks that cover the actual change using the owner's configured tools.
Use installed versions rather than assuming an API. Run relevant existing tests;
add regression coverage when behaviour changes and a useful seam exists.

Distinguish check-only commands from fix commands. A formatter passing does not
establish lint correctness, runtime behaviour or rendered equivalence. Keep
formatter exclusions separate where lint/syntax coverage remains useful.

Run independent checks even when another fails, then distinguish reported findings
from execution failures or unavailable prerequisites. Record commands, outcomes
and environmental limits. Never report a skipped or unrun check as passing.
Use synthetic fixtures or isolated services when tests write data; shared and
production data require separate explicit authorisation.
