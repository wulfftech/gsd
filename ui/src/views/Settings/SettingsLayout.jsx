import { Box, Container, Typography } from '@mui/joy'
const SettingsLayout = ({ title, children }) => {
  return (
    <Container>
      <Box sx={{ py: 2 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 3 }}>
          <Typography level='h2' sx={{ flexGrow: 1 }}>
            {title}
          </Typography>
        </Box>
        {children}
      </Box>
    </Container>
  )
}

export default SettingsLayout
