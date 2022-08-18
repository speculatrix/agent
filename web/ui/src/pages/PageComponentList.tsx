import { faCubes } from '@fortawesome/free-solid-svg-icons';
import Page from '../features/layout/Page';
import ComponentList from '../features/component/ComponentList';
import { testComponents } from '../testdata/data';

function PageComponentList() {
  return (
    <Page name="Components" desc="List of known components" icon={faCubes}>
      <ComponentList components={testComponents} />
    </Page>
  );
}

export default PageComponentList;
