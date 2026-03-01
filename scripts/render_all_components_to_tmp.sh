#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK_DIR="/tmp/mmdg-all-components"
INPUT_DIR="${WORK_DIR}/inputs"
BIN_DIR="${WORK_DIR}/bin"
REPORT_PATH="/tmp/mmdg_mmdc_all_components_report.txt"
REPORT_TMP="${REPORT_PATH}.tmp.$$"

mkdir -p "$INPUT_DIR" "$BIN_DIR"
: > "$REPORT_TMP"

if ! command -v mmdc >/dev/null 2>&1; then
  echo "error: mmdc not found in PATH" >&2
  exit 1
fi

pushd "$REPO_ROOT" >/dev/null
go build -o "$BIN_DIR/mmdg" ./cmd/mmdg
popd >/dev/null

write_fixture() {
  local name="$1"
  shift
  cat > "${INPUT_DIR}/${name}.mmd" <<EOF
$*
EOF
}

write_fixture "flowchart_complex" '
flowchart LR
  subgraph API["Public API"]
    A[Gateway] --> B{Auth?}
    B -->|yes| C[Route]
    B -->|no| D[Reject]
  end
  C --> E[(Users DB)]
  C --> F[[Audit Worker]]
  F -. async .-> G[(Audit Log)]
  E --> H((Done))
'

write_fixture "sequence_complex" '
sequenceDiagram
  participant U as User
  participant W as Web
  participant S as Service
  participant D as DB
  U->>W: Submit order
  W->>+S: validateAndCreate()
  alt valid
    par write order
      S->>+D: insert order
      D-->>-S: order id
    and publish event
      S-->>S: enqueue event
    end
    S-->>W: 201 Created
  else invalid
    S-->>W: 400 Bad Request
  end
  W-->>U: response
'

write_fixture "class_complex" '
classDiagram
  direction LR
  class Animal {
    +int age
    +String gender
    +isMammal()
    +mate()
  }
  class Duck {
    +String beakColor
    +swim()
    +quack()
  }
  class Fish {
    -int sizeInFeet
    -canEat()
  }
  class Zebra {
    +bool is_wild
    +run()
  }
  Animal <|-- Duck
  Animal <|-- Fish
  Animal <|-- Zebra
'

write_fixture "state_complex" '
stateDiagram-v2
  [*] --> Idle
  Idle --> Validate: submit
  state Validate {
    [*] --> CheckSchema
    CheckSchema --> CheckAuth
    CheckAuth --> [*]
  }
  Validate --> Approved: ok
  Validate --> Rejected: fail
  state Review <<choice>>
  Approved --> Review
  Review --> Fulfilled: auto
  Review --> Escalated: manual
  Escalated --> Fulfilled
  Fulfilled --> [*]
  Rejected --> [*]
'

write_fixture "er_complex" '
erDiagram
  CUSTOMER ||--o{ ORDER : places
  ORDER ||--|{ LINE_ITEM : contains
  CUSTOMER {
    string name
    string custNumber
    string sector
  }
  ORDER {
    int orderNumber
    string deliveryAddress
  }
  LINE_ITEM {
    string productCode
    int quantity
    float pricePerUnit
  }
'

write_fixture "pie_complex" '
pie showData
  title Product Revenue Mix
  "Subscriptions" : 55
  "Services" : 20
  "Marketplace" : 15
  "Training" : 10
'

write_fixture "mindmap_complex" '
mindmap
  root((Platform))
    Architecture
      API
      Workers
      Data
    Product
      Onboarding
      Billing
      Analytics
    Operations
      Observability
      Security
      Reliability
'

write_fixture "journey_complex" '
journey
  title Checkout experience
  section Discover
    Search product: 4: Alice, Bob
    Compare options: 3: Alice
  section Purchase
    Add to cart: 5: Alice
    Payment form: 2: Alice, Bob
    Confirmation: 4: Alice
'

write_fixture "timeline_complex" '
timeline
  title Platform milestones
  section Foundation
    2019 : MVP
    2020 : GA : First 100 customers
  section Scale
    2021 : Multi-region
    2022 : Enterprise controls : SSO
    2023 : AI features
'

write_fixture "gantt_complex" '
gantt
  title Release Train 2026
  dateFormat YYYY-MM-DD
  section Planning
    Scope freeze           :done, a1, 2026-01-01, 5d
    Architecture review    :done, a2, after a1, 4d
  section Build
    Feature implementation :active, b1, 2026-01-08, 12d
    Integration            :b2, after b1, 6d
  section Validation
    QA regression          :crit, c1, after b2, 7d
    Launch                 :milestone, m1, after c1, 0d
'

write_fixture "requirement_complex" '
requirementDiagram
  requirement test_req {
    id: 1
    text: the test text.
    risk: high
    verifymethod: test
  }
  functionalRequirement test_req2 {
    id: 1.1
    text: the second test text.
    risk: low
    verifymethod: inspection
  }
  element test_entity {
    type: simulation
  }
  test_entity - satisfies -> test_req2
  test_req - traces -> test_req2
'

write_fixture "gitgraph_complex" '
gitGraph
  commit id:"A0"
  branch develop
  checkout develop
  commit id:"D1"
  commit id:"D2"
  branch feature_payments
  checkout feature_payments
  commit id:"F1" type:HIGHLIGHT
  checkout develop
  merge feature_payments id:"M1" tag:"v1.1.0"
  checkout main
  commit id:"A1"
  merge develop id:"R1"
'

write_fixture "c4_context_complex" '
C4Context
  title Payments Platform Context
  Person(customer, "Customer")
  Person_Ext(support, "Support Agent")
  System(system, "Payments Platform", "Handles checkout and billing")
  System_Ext(bank, "Bank API", "External processor")
  Rel(customer, system, "Uses")
  Rel(system, bank, "Charges card", "HTTPS")
  Rel(support, system, "Investigates issues")
'

write_fixture "sankey_complex" '
sankey
Leads,Qualified,120
Qualified,Won,45
Qualified,Lost,75
Won,Expansion,20
Won,Churn,5
'

write_fixture "quadrant_complex" '
quadrantChart
  title Initiative Prioritization
  x-axis Low Effort --> High Effort
  y-axis Low Impact --> High Impact
  quadrant-1 Invest now
  quadrant-2 Stretch goals
  quadrant-3 Avoid
  quadrant-4 Plan later
  SSO rollout: [0.35, 0.85]
  Data residency: [0.78, 0.72]
  UI refresh: [0.42, 0.38]
  Cost optimization: [0.63, 0.44]
'

write_fixture "zenuml_complex" '
zenuml
  title Order orchestration
  Customer->Gateway: createOrder()
  Gateway->OrderService: create()
  OrderService->Inventory: reserve()
  if(reserved) {
    OrderService->Payment: charge()
    Payment->OrderService: ok
    @return
    OrderService->Customer: confirmed
  } else {
    OrderService->Customer: rejected
  }
'

write_fixture "block_complex" '
block
  columns 3
  A["Ingress"] B{"Validate"} C["Dispatch"]
  D["Retry Queue"] E[("DB")] F["Workers"]
  A --> B
  B --> C
  B --> D
  C --> E
  C --> F
'

write_fixture "packet_complex" '
packet
title TCP Header
0-15: "Source Port"
16-31: "Destination Port"
32-63: "Sequence Number"
64-95: "Acknowledgment Number"
96-99: "Data Offset"
100-103: "Flags"
104-119: "Window"
120-135: "Checksum"
136-151: "Urgent Pointer"
'

write_fixture "kanban_complex" '
kanban
  Backlog
    t1[Design auth flow]@{ ticket: SEC-101, assigned: "alice", priority: "High" }
    t2[Model threat scenarios]@{ assigned: "bob", priority: "Very High" }
  In Progress
    t3[Implement token rotation]
    t4[Add integration tests]@{ ticket: QA-77, assigned: "carol", priority: "Low" }
  Done
    t5[Document MFA rollout]
'

write_fixture "architecture_complex" '
architecture-beta
  group edge(cloud)[Edge]
  group core(cloud)[Core]
  service api(server)[API] in edge
  service auth(server)[Auth] in core
  service db(database)[DB] in core
  service queue(disk)[Queue] in core
  api:R -- L:auth
  auth:B -- T:db
  auth:R -- L:queue
'

write_fixture "radar_complex" '
radar-beta
  title Engineering Capability
  axis quality["Quality"], velocity["Velocity"], reliability["Reliability"]
  axis security["Security"], operability["Operability"], ux["UX"]
  curve teamA["Team A"]{82, 76, 88, 70, 79, 68}
  curve teamB["Team B"]{74, 84, 72, 81, 76, 73}
  max 100
  min 0
'

write_fixture "treemap_complex" '
treemap-beta
"Revenue"
  "SMB"
    "Self-serve": 140
    "Inside sales": 95
  "Enterprise"
    "New business": 180
    "Expansion": 130
  "Partners"
    "Resellers": 70
    "Marketplace": 55
'

write_fixture "xychart_complex" '
xychart-beta
  title "Revenue and Growth"
  x-axis [Q1, Q2, Q3, Q4, Q5, Q6]
  y-axis "ARR (k$)" 0 --> 240
  bar [40, 75, 110, 145, 180, 220]
  line [38, 70, 105, 142, 176, 215]
'

fixtures=(
  flowchart_complex
  sequence_complex
  class_complex
  state_complex
  er_complex
  pie_complex
  mindmap_complex
  journey_complex
  timeline_complex
  gantt_complex
  requirement_complex
  gitgraph_complex
  c4_context_complex
  sankey_complex
  quadrant_complex
  zenuml_complex
  block_complex
  packet_complex
  kanban_complex
  architecture_complex
  radar_complex
  treemap_complex
  xychart_complex
)

printf "component,mmdg_status,mmdc_status,mmdg_svg,mmdc_svg\n" >> "$REPORT_TMP"

for name in "${fixtures[@]}"; do
  input="${INPUT_DIR}/${name}.mmd"
  out_mmdg="/tmp/${name}_mmdg.svg"
  out_mmdc="/tmp/${name}_mmdc.svg"
  mmdg_status="ok"
  mmdc_status="ok"

  if ! "$BIN_DIR/mmdg" -i "$input" -o "$out_mmdg" --allowApproximate >/tmp/${name}_mmdg.log 2>&1; then
    mmdg_status="fail"
  fi

  if ! mmdc -i "$input" -o "$out_mmdc" -e svg -w 2200 -H 1600 -b white -q >/tmp/${name}_mmdc.log 2>&1; then
    mmdc_status="fail"
  fi

  printf "%s,%s,%s,%s,%s\n" "$name" "$mmdg_status" "$mmdc_status" "$out_mmdg" "$out_mmdc" >> "$REPORT_TMP"
done

mapfile -t fidelity_files < <(rg --files "$REPO_ROOT/testdata/fidelity" --glob '*.mmd' | sort)
for input in "${fidelity_files[@]}"; do
  name="$(basename "${input%.mmd}")"
  out_mmdg="/tmp/${name}_mmdg.svg"
  out_mmdc="/tmp/${name}_mmdc.svg"
  mmdg_status="ok"
  mmdc_status="ok"

  if ! "$BIN_DIR/mmdg" -i "$input" -o "$out_mmdg" --allowApproximate >/tmp/${name}_mmdg.log 2>&1; then
    mmdg_status="fail"
  fi

  if ! mmdc -i "$input" -o "$out_mmdc" -e svg -w 2200 -H 1600 -b white -q >/tmp/${name}_mmdc.log 2>&1; then
    mmdc_status="fail"
  fi

  printf "%s,%s,%s,%s,%s\n" "$name" "$mmdg_status" "$mmdc_status" "$out_mmdg" "$out_mmdc" >> "$REPORT_TMP"
done

mv "$REPORT_TMP" "$REPORT_PATH"
echo "render report: $REPORT_PATH"
echo "generated svg pairs in /tmp/{name}_mmdg.svg and /tmp/{name}_mmdc.svg"
