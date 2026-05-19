# AGENTS.md

## What this is

SugarCRM module for ProcessMaker BPMN (Business Process Model and Notation). Installs into a SugarCRM instance under `modules/`. Not a standalone app — no `package.json`, `composer.json`, or local build/test tooling.

## Module structure

| Directory | Purpose |
|---|---|
| `pmse_Inbox/` | Core BPMN engine — case management, workflow execution, REST APIs |
| `pmse_Project/` | Process definitions — BPMN elements (flows, gateways, events, forms, participants, lanes, etc.) |
| `pmse_Business_Rules/` | Business rule definitions and export/import |
| `pmse_Emails_Templates/` | Email template definitions for workflow notifications |

## Key architecture

- **Engine core**: `pmse_Inbox/engine/PMSE.php` — singleton, registers all module paths
- **Workflow execution**: `pmse_Inbox/engine/PMSEExecuter.php`, `PMSEFlowRouter.php`
- **REST APIs**: `pmse_Inbox/clients/base/api/` — `PMSEEngineApi`, `PMSEEngineFilterApi`, `PMSEImageGeneratorApi`, `PMSECasesListApi`
- **Controller**: `pmse_Inbox/controller.php` — SugarController subclass with actions for routing cases, showing process images (`action_showPNG`), case history, notes
- **Factory pattern**: `ProcessManager\Factory::getPMSEObject()` — use this to instantiate engine objects, never `new` directly
- **Logic hooks**: `pmse_Inbox/engine/PMSELogicHook.php` — SugarCRM event hooks for workflow triggers

## SugarCRM conventions (critical)

- **Every PHP file** starts with `if(!defined('sugarEntry') || !sugarEntry) die('Not A Valid Entry Point');` — do not remove
- `*_sugar.php` files are **auto-generated** by SugarCRM Module Builder — do not edit directly; extend them in the non-`_sugar` counterpart
- `vardefs.php` defines bean schema and field metadata — changes require a **Quick Repair & Rebuild** in SugarCRM admin
- `metadata/` contains view definitions (detail, edit, list, search, popup, dashlet)
- `language/` contains 38 locale files — naming is `<lang>_<region>.lang.php`
- `clients/base/` follows SugarCRM Sidecar framework structure (api, fields, filters, layouts, menus, plugins, views)
- `clients/mobile/` for mobile-specific overrides

## Upgrade/install flow

- `pmse_Project/upgrade/scripts/post/` — post-install/upgrade scripts
- Module is distributed as a SugarCRM installable package (zip), not via composer/npm

## How to verify changes

There is no local test runner. Verification is done by:
1. Packaging the module as a SugarCRM installable zip
2. Installing into a SugarCRM instance
3. Running **Admin > Repair > Quick Repair & Rebuild** after any vardefs/metadata changes
4. Testing workflows through the SugarCRM UI or REST API

## Gotchas

- `PMSE.php` lists 5 module paths including `pmse_Config` (not present in this repo — may be legacy or in a separate package)
- `action_routeCase` in controller is marked `@deprecated since version pmse2` — do not extend
- Case statuses use string values: `IN PROGRESS`, default PIN is `0000`, assigned status defaults to `UNASSIGNED`
