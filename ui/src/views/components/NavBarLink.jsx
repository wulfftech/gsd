import {
  Chip,
  ListItem,
  ListItemButton,
  ListItemContent,
  ListItemDecorator,
} from '@mui/joy'
import { Link } from 'react-router-dom'

const NavBarLink = ({ link }) => {
  const { to, icon, label, badge } = link
  return (
    <ListItem>
      <ListItemButton
        key={to}
        component={Link}
        to={to}
        variant='plain'
        color='neutral'
        sx={{
          borderRadius: 4,
          py: 1.2,
        }}
      >
        <ListItemDecorator>{icon}</ListItemDecorator>
        <ListItemContent>{label}</ListItemContent>
        {badge > 0 && (
          <Chip size='sm' color='warning' variant='solid'>
            {badge}
          </Chip>
        )}
      </ListItemButton>
    </ListItem>
  )
}

export default NavBarLink
