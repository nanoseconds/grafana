import React from 'react';
import cn from 'classnames';
import { HeaderGroup, Column } from 'react-table';
import { DataFrame, Field } from '@grafana/data'; // yf

import { selectors } from '@grafana/e2e-selectors';

import { useStyles2 } from '../../themes';
import { getFieldTypeIcon } from '../../types';
import { Icon } from '../Icon/Icon';

import { Filter } from './Filter';
import { getTableStyles, TableStyles } from './styles';

export interface HeaderRowProps {
  headerGroups: HeaderGroup[];
  showTypeIcons?: boolean;
  showRowNum?: boolean; // yf 
  data: DataFrame; // yf 
}

export const HeaderRow = (props: HeaderRowProps) => {
  const { headerGroups, data, showTypeIcons, showRowNum } = props;
  const e2eSelectorsTable = selectors.components.Panels.Visualization.Table;
  const tableStyles = useStyles2(getTableStyles);

  return (
    <div role="rowgroup">
      {headerGroups.map((headerGroup: HeaderGroup) => {
        const { key, ...headerGroupProps } = headerGroup.getHeaderGroupProps();
        return (
          <div
            className={tableStyles.thead}
            {...headerGroupProps}
            key={key}
            aria-label={e2eSelectorsTable.header}
            role="row"
          >
            {headerGroup.headers.map((column: Column, index: number) =>
              renderHeaderCell2(column, tableStyles, data.fields[index - (showRowNum ? 1 : 0)], showTypeIcons)
            )}
          </div>
        );
      })}
    </div>
  );
};

function renderHeaderCell(column: any, tableStyles: TableStyles, showTypeIcons?: boolean) {
  const headerProps = column.getHeaderProps();
  const field = column.field ?? null;

  if (column.canResize) {
    headerProps.style.userSelect = column.isResizing ? 'none' : 'auto'; // disables selecting text while resizing
  }

  headerProps.style.position = 'absolute';
  headerProps.style.justifyContent = (column as any).justifyContent;

  let link = null;
  if (field && field?.getHeaderLinks) {
    link = field.getHeaderLinks({})[0];
  }

  if (!!link) {
    return (
      <div className={cn(tableStyles.headerCell, tableStyles.cellLink)} {...headerProps}>
        <a href={link.href} target={link.target} title={link.title}>
          {column.render('Header')}
        </a>
      </div>
    );
  }

  return (
    <div className={tableStyles.headerCell} {...headerProps} role="columnheader">
      {column.canSort && (
        <>
          <button {...column.getSortByToggleProps()} className={tableStyles.headerCellLabel}>
            {showTypeIcons && (
              <Icon name={getFieldTypeIcon(field)} title={field?.type} size="sm" className={tableStyles.typeIcon} />
            )}
            <div>{column.render('Header')}</div>
            <div>
              {column.isSorted && (column.isSortedDesc ? <Icon name="arrow-down" /> : <Icon name="arrow-up" />)}
            </div>
          </button>
          {column.canFilter && <Filter column={column} tableStyles={tableStyles} field={field} />}
        </>
      )}
      {!column.canSort && column.render('Header')}
      {!column.canSort && column.canFilter && <Filter column={column} tableStyles={tableStyles} field={field} />}
      {column.canResize && <div {...column.getResizerProps()} className={tableStyles.resizeHandle} />}
    </div>
  );
}


// yf
function renderHeaderCell2(column: any, tableStyles: TableStyles, field?: Field, showTypeIcons?: boolean) {
  const headerProps = column.getHeaderProps();

  if (column.canResize) {
    headerProps.style.userSelect = column.isResizing ? 'none' : 'auto'; // disables selecting text while resizing
  }

  headerProps.style.position = 'absolute';
  headerProps.style.justifyContent = (column as any).justifyContent;

  let link = null;
  if (field && field?.getHeaderLinks) {
    link = field.getHeaderLinks({})[0];
  }

  if (!!link) {
    return (
      <div className={cn(tableStyles.headerCell, tableStyles.cellLink)} {...headerProps}>
        <a href={link.href} target={link.target} title={link.title}>
          {column.render('Header')}
        </a>
      </div>
    );
  }

  return (
    <div className={tableStyles.headerCell} {...headerProps} role="columnheader">
      {column.canSort && (
        <>
          <button {...column.getSortByToggleProps()} className={tableStyles.headerCellLabel}>
            {showTypeIcons && (
              <Icon name={getFieldTypeIcon(field)} title={field?.type} size="sm" className={tableStyles.typeIcon} />
            )}
            <div>{column.render('Header')}</div>
            <div>
              {column.isSorted && (column.isSortedDesc ? <Icon name="arrow-down" /> : <Icon name="arrow-up" />)}
            </div>
          </button>
          {column.canFilter && <Filter column={column} tableStyles={tableStyles} field={field} />}
        </>
      )}
      {!column.canSort && column.render('Header')}
      {!column.canSort && column.canFilter && <Filter column={column} tableStyles={tableStyles} field={field} />}
      {column.canResize && <div {...column.getResizerProps()} className={tableStyles.resizeHandle} />}
    </div>
  );
}