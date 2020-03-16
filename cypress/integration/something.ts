/// <reference path="../../node_modules/@grafana/e2e/cypress/support/index.d.ts" />
import { addDataSource, addPanel, getByPlaceholder } from './temp';
import { e2e } from '@grafana/e2e';

export const addDashboard = () => {
  // These get auto-removed within `afterEach` of @grafana/e2e
  addDataSource({
    alertMessage: 'Success',
    beforeSubmit: () => {
      getByPlaceholder('Enter API Key').scrollIntoView().type('abc123');
    },
    name: 'Google Sheets',
  });

  e2e.flows.addDashboard();
};

const something = () => {
  addPanel({
    dataSourceName: 'Google Sheets',
  });
};

e2e.scenario({
  describeName: 'Smoke tests',
  itName: 'Login, create data source, dashboard and panel',
  scenario: () => {
    addDashboard();
    something();
  },
});
