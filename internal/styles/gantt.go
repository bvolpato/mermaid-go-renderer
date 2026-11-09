// Ported from mermaid 12.2.0, mermaid/src/diagrams/gantt/styles.js (MIT).

package styles

// ganttStyles is getStyles of the gantt diagram: the CSS of the diagram type.
func ganttStyles(o *options) string {
	return `
  .mermaid-main-font {
        font-family: ` + o.s("fontFamily") + `;
  }

  .exclude-range {
    fill: ` + o.s("excludeBkgColor") + `;
  }

  .section {
    stroke: none;
    opacity: 0.2;
  }

  .section0 {
    fill: ` + o.s("sectionBkgColor") + `;
  }

  .section2 {
    fill: ` + o.s("sectionBkgColor2") + `;
  }

  .section1,
  .section3 {
    fill: ` + o.s("altSectionBkgColor") + `;
    opacity: 0.2;
  }

  .sectionTitle0 {
    fill: ` + o.s("titleColor") + `;
  }

  .sectionTitle1 {
    fill: ` + o.s("titleColor") + `;
  }

  .sectionTitle2 {
    fill: ` + o.s("titleColor") + `;
  }

  .sectionTitle3 {
    fill: ` + o.s("titleColor") + `;
  }

  .sectionTitle {
    text-anchor: start;
    font-family: ` + o.s("fontFamily") + `;
  }


  /* Grid and axis */

  .grid .tick {
    stroke: ` + o.s("gridColor") + `;
    opacity: 0.8;
    shape-rendering: crispEdges;
  }

  .grid .tick text {
    font-family: ` + o.s("fontFamily") + `;
    fill: ` + o.s("textColor") + `;
  }

  .grid path {
    stroke-width: 0;
  }


  /* Today line */

  .today {
    fill: none;
    stroke: ` + o.s("todayLineColor") + `;
    stroke-width: 2px;
  }


  /* Task styling */

  /* Default task */

  .task {
    stroke-width: 2;
  }

  .taskText {
    text-anchor: middle;
    font-family: ` + o.s("fontFamily") + `;
  }

  .taskTextOutsideRight {
    fill: ` + o.s("taskTextDarkColor") + `;
    text-anchor: start;
    font-family: ` + o.s("fontFamily") + `;
  }

  .taskTextOutsideLeft {
    fill: ` + o.s("taskTextDarkColor") + `;
    text-anchor: end;
  }


  /* Special case clickable */

  .task.clickable {
    cursor: pointer;
  }

  .taskText.clickable {
    cursor: pointer;
    fill: ` + o.s("taskTextClickableColor") + ` !important;
    font-weight: bold;
  }

  .taskTextOutsideLeft.clickable {
    cursor: pointer;
    fill: ` + o.s("taskTextClickableColor") + ` !important;
    font-weight: bold;
  }

  .taskTextOutsideRight.clickable {
    cursor: pointer;
    fill: ` + o.s("taskTextClickableColor") + ` !important;
    font-weight: bold;
  }


  /* Specific task settings for the sections*/

  .taskText0,
  .taskText1,
  .taskText2,
  .taskText3 {
    fill: ` + o.s("taskTextColor") + `;
  }

  .task0,
  .task1,
  .task2,
  .task3 {
    fill: ` + o.s("taskBkgColor") + `;
    stroke: ` + o.s("taskBorderColor") + `;
  }

  .taskTextOutside0,
  .taskTextOutside2
  {
    fill: ` + o.s("taskTextOutsideColor") + `;
  }

  .taskTextOutside1,
  .taskTextOutside3 {
    fill: ` + o.s("taskTextOutsideColor") + `;
  }


  /* Active task */

  .active0,
  .active1,
  .active2,
  .active3 {
    fill: ` + o.s("activeTaskBkgColor") + `;
    stroke: ` + o.s("activeTaskBorderColor") + `;
  }

  .activeText0,
  .activeText1,
  .activeText2,
  .activeText3 {
    fill: ` + o.s("taskTextDarkColor") + ` !important;
  }


  /* Completed task */

  .done0,
  .done1,
  .done2,
  .done3 {
    stroke: ` + o.s("doneTaskBorderColor") + `;
    fill: ` + o.s("doneTaskBkgColor") + `;
    stroke-width: 2;
  }

  .doneText0,
  .doneText1,
  .doneText2,
  .doneText3 {
    fill: ` + o.s("taskTextDarkColor") + ` !important;
  }

  /* Done task text displayed outside the bar sits against the diagram background,
     not against the done-task bar, so it must use the outside/contrast color. */
  .doneText0.taskTextOutsideLeft,
  .doneText0.taskTextOutsideRight,
  .doneText1.taskTextOutsideLeft,
  .doneText1.taskTextOutsideRight,
  .doneText2.taskTextOutsideLeft,
  .doneText2.taskTextOutsideRight,
  .doneText3.taskTextOutsideLeft,
  .doneText3.taskTextOutsideRight {
    fill: ` + o.s("taskTextOutsideColor") + ` !important;
  }


  /* Tasks on the critical line */

  .crit0,
  .crit1,
  .crit2,
  .crit3 {
    stroke: ` + o.s("critBorderColor") + `;
    fill: ` + o.s("critBkgColor") + `;
    stroke-width: 2;
  }

  .activeCrit0,
  .activeCrit1,
  .activeCrit2,
  .activeCrit3 {
    stroke: ` + o.s("critBorderColor") + `;
    fill: ` + o.s("activeTaskBkgColor") + `;
    stroke-width: 2;
  }

  .doneCrit0,
  .doneCrit1,
  .doneCrit2,
  .doneCrit3 {
    stroke: ` + o.s("critBorderColor") + `;
    fill: ` + o.s("doneTaskBkgColor") + `;
    stroke-width: 2;
    cursor: pointer;
    shape-rendering: crispEdges;
  }

  .milestone {
    transform: rotate(45deg) scale(0.8,0.8);
  }

  .milestoneText {
    font-style: italic;
  }
  .doneCritText0,
  .doneCritText1,
  .doneCritText2,
  .doneCritText3 {
    fill: ` + o.s("taskTextDarkColor") + ` !important;
  }

  /* Done-crit task text outside the bar — same reasoning as doneText above. */
  .doneCritText0.taskTextOutsideLeft,
  .doneCritText0.taskTextOutsideRight,
  .doneCritText1.taskTextOutsideLeft,
  .doneCritText1.taskTextOutsideRight,
  .doneCritText2.taskTextOutsideLeft,
  .doneCritText2.taskTextOutsideRight,
  .doneCritText3.taskTextOutsideLeft,
  .doneCritText3.taskTextOutsideRight {
    fill: ` + o.s("taskTextOutsideColor") + ` !important;
  }

  .vert {
    stroke: ` + o.s("vertLineColor") + `;
  }

  .vertText {
    font-size: 15px;
    text-anchor: middle;
    fill: ` + o.s("vertLineColor") + ` !important;
  }

  .activeCritText0,
  .activeCritText1,
  .activeCritText2,
  .activeCritText3 {
    fill: ` + o.s("taskTextDarkColor") + ` !important;
  }

  .titleText {
    text-anchor: middle;
    font-size: 18px;
    fill: ` + s(or(o.Get("titleColor"), o.Get("textColor"))) + `;
    font-family: ` + o.s("fontFamily") + `;
  }
`
}
