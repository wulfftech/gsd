// Static insight filter definitions – used for URL restoration
export const INSIGHT_FILTER_DEFS = {
  overdue: {
    name: 'Overdue',
    filter: {
      conditions: [{ type: 'dueDate', operator: 'isOverdue', value: null }],
      operator: 'AND',
    },
  },
  'due-today': {
    name: 'Due Today',
    filter: {
      conditions: [{ type: 'dueDate', operator: 'isDueToday', value: null }],
      operator: 'AND',
    },
  },
  'pending-approval': {
    name: 'Pending Approval',
    filter: {
      conditions: [{ type: 'status', operator: 'is', value: 3 }],
      operator: 'AND',
    },
  },
  'due-this-week': {
    name: 'Due This Week',
    filter: {
      conditions: [{ type: 'dueDate', operator: 'isDueThisWeek', value: null }],
      operator: 'AND',
    },
  },
  'high-priority': {
    name: 'High Priority',
    filter: {
      conditions: [{ type: 'priority', operator: 'is', value: [1, 2] }],
      operator: 'AND',
    },
  },
  'no-due-date': {
    name: 'No Due Date',
    filter: {
      conditions: [{ type: 'dueDate', operator: 'hasNoDueDate', value: null }],
      operator: 'AND',
    },
  },
}
